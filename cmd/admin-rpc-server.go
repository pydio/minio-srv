/*
 * Minio Cloud Storage, (C) 2016, 2017 Minio, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	router "github.com/gorilla/mux"
)

const adminPath = "/admin"

var errUnsupportedBackend = errors.New("not supported for non erasure-code backend")

// adminCmd - exports RPC methods for service status, stop and
// restart commands.
type adminCmd struct {
	AuthRPCServer
}

// ListLocksQuery - wraps ListLocks API's query values to send over RPC.
type ListLocksQuery struct {
	AuthRPCArgs
	bucket   string
	prefix   string
	duration time.Duration
}

// ListLocksReply - wraps ListLocks response over RPC.
type ListLocksReply struct {
	AuthRPCReply
	volLocks []VolumeLockInfo
}

// ServerInfoDataReply - wraps the server info response over RPC.
type ServerInfoDataReply struct {
	AuthRPCReply
	ServerInfoData ServerInfoData
}

// ConfigReply - wraps the server config response over RPC.
type ConfigReply struct {
	AuthRPCReply
	Config []byte // json-marshalled bytes of serverConfigV13
}

// Restart - Restart this instance of minio server.
func (s *adminCmd) Restart(args *AuthRPCArgs, reply *AuthRPCReply) error {
	if err := args.IsAuthenticated(); err != nil {
		return err
	}

	globalServiceSignalCh <- serviceRestart
	return nil
}

// ListLocks - lists locks held by requests handled by this server instance.
func (s *adminCmd) ListLocks(query *ListLocksQuery, reply *ListLocksReply) error {
	if err := query.IsAuthenticated(); err != nil {
		return err
	}
	volLocks := listLocksInfo(query.bucket, query.prefix, query.duration)
	*reply = ListLocksReply{volLocks: volLocks}
	return nil
}

// ReInitDisk - reinitialize storage disks and object layer to use the
// new format.
func (s *adminCmd) ReInitDisks(args *AuthRPCArgs, reply *AuthRPCReply) error {
	if err := args.IsAuthenticated(); err != nil {
		return err
	}

	if !globalIsXL {
		return errUnsupportedBackend
	}

	// Get the current object layer instance.
	objLayer := newObjectLayerFn()

	// Initialize new disks to include the newly formatted disks.
	bootstrapDisks, err := initStorageDisks(globalEndpoints)
	if err != nil {
		return err
	}

	// Initialize new object layer with newly formatted disks.
	newObjectAPI, err := newXLObjects(bootstrapDisks)
	if err != nil {
		return err
	}

	// Replace object layer with newly formatted storage.
	globalObjLayerMutex.Lock()
	globalObjectAPI = newObjectAPI
	globalObjLayerMutex.Unlock()

	// Shutdown storage belonging to old object layer instance.
	objLayer.Shutdown()

	return nil
}

// ServerInfo - returns the server info when object layer was initialized on this server.
func (s *adminCmd) ServerInfoData(args *AuthRPCArgs, reply *ServerInfoDataReply) error {
	if err := args.IsAuthenticated(); err != nil {
		return err
	}

	if globalBootTime.IsZero() {
		return errServerNotInitialized
	}

	// Build storage info
	objLayer := newObjectLayerFn()
	if objLayer == nil {
		return errServerNotInitialized
	}
	storageInfo := objLayer.StorageInfo()

	var arns []string
	for queueArn := range globalEventNotifier.GetAllExternalTargets() {
		arns = append(arns, queueArn)
	}

	reply.ServerInfoData = ServerInfoData{
		Properties: ServerProperties{
			Uptime:   UTCNow().Sub(globalBootTime),
			Version:  Version,
			CommitID: CommitID,
			Region:   serverConfig.GetRegion(),
			SQSARN:   arns,
		},
		StorageInfo: storageInfo,
		ConnStats:   globalConnStats.toServerConnStats(),
		HTTPStats:   globalHTTPStats.toServerHTTPStats(),
	}

	return nil
}

// GetConfig - returns the config.json of this server.
func (s *adminCmd) GetConfig(args *AuthRPCArgs, reply *ConfigReply) error {
	if err := args.IsAuthenticated(); err != nil {
		return err
	}

	if serverConfig == nil {
		return errors.New("config not present")
	}

	jsonBytes, err := json.Marshal(serverConfig)
	if err != nil {
		return err
	}

	reply.Config = jsonBytes
	return nil
}

// WriteConfigArgs - wraps the bytes to be written and temporary file name.
type WriteConfigArgs struct {
	AuthRPCArgs
	TmpFileName string
	Buf         []byte
}

// WriteConfigReply - wraps the result of a writing config into a temporary file.
// the remote node.
type WriteConfigReply struct {
	AuthRPCReply
}

func writeTmpConfigCommon(tmpFileName string, configBytes []byte) error {
	tmpConfigFile := filepath.Join(getConfigDir(), tmpFileName)
	err := ioutil.WriteFile(tmpConfigFile, configBytes, 0666)
	errorIf(err, fmt.Sprintf("Failed to write to temporary config file %s", tmpConfigFile))
	return err
}

// WriteTmpConfig - writes the supplied config contents onto the
// supplied temporary file.
func (s *adminCmd) WriteTmpConfig(wArgs *WriteConfigArgs, wReply *WriteConfigReply) error {
	if err := wArgs.IsAuthenticated(); err != nil {
		return err
	}

	return writeTmpConfigCommon(wArgs.TmpFileName, wArgs.Buf)
}

// CommitConfigArgs - wraps the config file name that needs to be
// committed into config.json on this node.
type CommitConfigArgs struct {
	AuthRPCArgs
	FileName string
}

// CommitConfigReply - represents response to commit of config file on
// this node.
type CommitConfigReply struct {
	AuthRPCReply
}

// CommitConfig - Renames the temporary file into config.json on this node.
func (s *adminCmd) CommitConfig(cArgs *CommitConfigArgs, cReply *CommitConfigReply) error {
	configFile := getConfigFile()
	tmpConfigFile := filepath.Join(getConfigDir(), cArgs.FileName)

	err := os.Rename(tmpConfigFile, configFile)
	errorIf(err, fmt.Sprintf("Failed to rename %s to %s", tmpConfigFile, configFile))
	return err
}

// registerAdminRPCRouter - registers RPC methods for service status,
// stop and restart commands.
func registerAdminRPCRouter(mux *router.Router) error {
	adminRPCHandler := &adminCmd{}
	adminRPCServer := newRPCServer()
	err := adminRPCServer.RegisterName("Admin", adminRPCHandler)
	if err != nil {
		return traceError(err)
	}
	adminRouter := mux.NewRoute().PathPrefix(minioReservedBucketPath).Subrouter()
	adminRouter.Path(adminPath).Handler(adminRPCServer)
	return nil
}
