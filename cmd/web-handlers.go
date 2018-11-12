/*
 * Minio Cloud Storage, (C) 2016, 2017, 2018 Minio, Inc.
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
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	humanize "github.com/dustin/go-humanize"
	snappy "github.com/golang/snappy"
	"github.com/gorilla/mux"
	"github.com/gorilla/rpc/v2/json2"
	miniogopolicy "github.com/pydio/minio-go/pkg/policy"
	"github.com/pydio/minio-go/pkg/s3utils"
	"github.com/pydio/minio-srv/browser"
	"github.com/pydio/minio-srv/cmd/crypto"
	"github.com/pydio/minio-srv/cmd/logger"
	"github.com/pydio/minio-srv/pkg/auth"
	"github.com/pydio/minio-srv/pkg/dns"
	"github.com/pydio/minio-srv/pkg/event"
	"github.com/pydio/minio-srv/pkg/handlers"
	"github.com/pydio/minio-srv/pkg/hash"
	"github.com/pydio/minio-srv/pkg/iam/policy"
	"github.com/pydio/minio-srv/pkg/ioutil"
	"github.com/pydio/minio-srv/pkg/policy"
)

// WebGenericArgs - empty struct for calls that don't accept arguments
// for ex. ServerInfo, GenerateAuth
type WebGenericArgs struct{}

// WebGenericRep - reply structure for calls for which reply is success/failure
// for ex. RemoveObject MakeBucket
type WebGenericRep struct {
	UIVersion string `json:"uiVersion"`
}

// ServerInfoRep - server info reply.
type ServerInfoRep struct {
	MinioVersion    string
	MinioMemory     string
	MinioPlatform   string
	MinioRuntime    string
	MinioGlobalInfo map[string]interface{}
	UIVersion       string `json:"uiVersion"`
}

// ServerInfo - get server info.
func (web *webAPIHandlers) ServerInfo(r *http.Request, args *WebGenericArgs, reply *ServerInfoRep) error {
	_, owner, authErr := webRequestAuthenticate(r)
	if authErr != nil {
		return toJSONError(authErr)
	}
	host, err := os.Hostname()
	if err != nil {
		host = ""
	}
	memstats := &runtime.MemStats{}
	runtime.ReadMemStats(memstats)
	mem := fmt.Sprintf("Used: %s | Allocated: %s | Used-Heap: %s | Allocated-Heap: %s",
		humanize.Bytes(memstats.Alloc),
		humanize.Bytes(memstats.TotalAlloc),
		humanize.Bytes(memstats.HeapAlloc),
		humanize.Bytes(memstats.HeapSys))
	platform := fmt.Sprintf("Host: %s | OS: %s | Arch: %s",
		host,
		runtime.GOOS,
		runtime.GOARCH)
	goruntime := fmt.Sprintf("Version: %s | CPUs: %s", runtime.Version(), strconv.Itoa(runtime.NumCPU()))

	reply.MinioVersion = Version
	reply.MinioGlobalInfo = getGlobalInfo()
	// If ENV creds are not set and incoming user is not owner
	// disable changing credentials.
	// TODO: fix this in future and allow changing user credentials.
	v, ok := reply.MinioGlobalInfo["isEnvCreds"].(bool)
	if ok && !v {
		reply.MinioGlobalInfo["isEnvCreds"] = !owner
	}

	reply.MinioMemory = mem
	reply.MinioPlatform = platform
	reply.MinioRuntime = goruntime
	reply.UIVersion = browser.UIVersion
	return nil
}

// StorageInfoRep - contains storage usage statistics.
type StorageInfoRep struct {
	StorageInfo StorageInfo `json:"storageInfo"`
	UIVersion   string      `json:"uiVersion"`
}

// StorageInfo - web call to gather storage usage statistics.
func (web *webAPIHandlers) StorageInfo(r *http.Request, args *AuthArgs, reply *StorageInfoRep) error {
	objectAPI := web.ObjectAPI()
	if objectAPI == nil {
		return toJSONError(errServerNotInitialized)
	}
	_, _, authErr := webRequestAuthenticate(r)
	if authErr != nil {
		return toJSONError(authErr)
	}
	reply.StorageInfo = objectAPI.StorageInfo(context.Background())
	reply.UIVersion = browser.UIVersion
	return nil
}

// MakeBucketArgs - make bucket args.
type MakeBucketArgs struct {
	BucketName string `json:"bucketName"`
}

// MakeBucket - creates a new bucket.
func (web *webAPIHandlers) MakeBucket(r *http.Request, args *MakeBucketArgs, reply *WebGenericRep) error {
	objectAPI := web.ObjectAPI()
	if objectAPI == nil {
		return toJSONError(errServerNotInitialized)
	}
	_, owner, authErr := webRequestAuthenticate(r)
	if authErr != nil {
		return toJSONError(authErr)
	}
	// TODO: Allow MakeBucket in future.
	if !owner {
		return toJSONError(errAccessDenied)
	}

	// Check if bucket is a reserved bucket name or invalid.
	if isReservedOrInvalidBucket(args.BucketName) {
		return toJSONError(errInvalidBucketName)
	}

	if globalDNSConfig != nil {
		if _, err := globalDNSConfig.Get(args.BucketName); err != nil {
			if err == dns.ErrNoEntriesFound {
				// Proceed to creating a bucket.
				if err = objectAPI.MakeBucketWithLocation(context.Background(), args.BucketName, globalServerConfig.GetRegion()); err != nil {
					return toJSONError(err)
				}
				if err = globalDNSConfig.Put(args.BucketName); err != nil {
					objectAPI.DeleteBucket(context.Background(), args.BucketName)
					return toJSONError(err)
				}

				reply.UIVersion = browser.UIVersion
				return nil
			}
			return toJSONError(err)
		}
		return toJSONError(errBucketAlreadyExists)
	}

	if err := objectAPI.MakeBucketWithLocation(context.Background(), args.BucketName, globalServerConfig.GetRegion()); err != nil {
		return toJSONError(err, args.BucketName)
	}

	reply.UIVersion = browser.UIVersion
	return nil
}

// RemoveBucketArgs - remove bucket args.
type RemoveBucketArgs struct {
	BucketName string `json:"bucketName"`
}

// DeleteBucket - removes a bucket, must be empty.
func (web *webAPIHandlers) DeleteBucket(r *http.Request, args *RemoveBucketArgs, reply *WebGenericRep) error {
	objectAPI := web.ObjectAPI()
	if objectAPI == nil {
		return toJSONError(errServerNotInitialized)
	}
	_, owner, authErr := webRequestAuthenticate(r)
	if authErr != nil {
		return toJSONError(authErr)
	}
	// TODO: Allow DeleteBucket in future.
	if !owner {
		return toJSONError(errAccessDenied)
	}

	ctx := context.Background()

	deleteBucket := objectAPI.DeleteBucket
	if web.CacheAPI() != nil {
		deleteBucket = web.CacheAPI().DeleteBucket
	}

	if err := deleteBucket(ctx, args.BucketName); err != nil {
		return toJSONError(err, args.BucketName)
	}

	globalNotificationSys.RemoveNotification(args.BucketName)
	globalPolicySys.Remove(args.BucketName)
	globalNotificationSys.DeleteBucket(ctx, args.BucketName)

	if globalDNSConfig != nil {
		if err := globalDNSConfig.Delete(args.BucketName); err != nil {
			// Deleting DNS entry failed, attempt to create the bucket again.
			objectAPI.MakeBucketWithLocation(ctx, args.BucketName, "")
			return toJSONError(err)
		}
	}

	reply.UIVersion = browser.UIVersion
	return nil
}

// ListBucketsRep - list buckets response
type ListBucketsRep struct {
	Buckets   []WebBucketInfo `json:"buckets"`
	UIVersion string          `json:"uiVersion"`
}

// WebBucketInfo container for list buckets metadata.
type WebBucketInfo struct {
	// The name of the bucket.
	Name string `json:"name"`
	// Date the bucket was created.
	CreationDate time.Time `json:"creationDate"`
}

// ListBuckets - list buckets api.
func (web *webAPIHandlers) ListBuckets(r *http.Request, args *WebGenericArgs, reply *ListBucketsRep) error {
	objectAPI := web.ObjectAPI()
	if objectAPI == nil {
		return toJSONError(errServerNotInitialized)
	}
	listBuckets := objectAPI.ListBuckets
	if web.CacheAPI() != nil {
		listBuckets = web.CacheAPI().ListBuckets
	}

	if _, _, authErr := webRequestAuthenticate(r); authErr != nil {
		return toJSONError(authErr)
	}

	// If etcd, dns federation configured list buckets from etcd.
	if globalDNSConfig != nil {
		dnsBuckets, err := globalDNSConfig.List()
		if err != nil {
			return toJSONError(err)
		}
		for _, dnsRecord := range dnsBuckets {
			bucketName := strings.Trim(dnsRecord.Key, "/")
			reply.Buckets = append(reply.Buckets, WebBucketInfo{
				Name:         bucketName,
				CreationDate: dnsRecord.CreationDate,
			})
		}
	} else {
		buckets, err := listBuckets(context.Background())
		if err != nil {
			return toJSONError(err)
		}
		for _, bucket := range buckets {
			reply.Buckets = append(reply.Buckets, WebBucketInfo{
				Name:         bucket.Name,
				CreationDate: bucket.Created,
			})
		}
	}

	reply.UIVersion = browser.UIVersion
	return nil
}

// ListObjectsArgs - list object args.
type ListObjectsArgs struct {
	BucketName string `json:"bucketName"`
	Prefix     string `json:"prefix"`
	Marker     string `json:"marker"`
}

// ListObjectsRep - list objects response.
type ListObjectsRep struct {
	Objects     []WebObjectInfo `json:"objects"`
	NextMarker  string          `json:"nextmarker"`
	IsTruncated bool            `json:"istruncated"`
	Writable    bool            `json:"writable"` // Used by client to show "upload file" button.
	UIVersion   string          `json:"uiVersion"`
}

// WebObjectInfo container for list objects metadata.
type WebObjectInfo struct {
	// Name of the object
	Key string `json:"name"`
	// Date and time the object was last modified.
	LastModified time.Time `json:"lastModified"`
	// Size in bytes of the object.
	Size int64 `json:"size"`
	// ContentType is mime type of the object.
	ContentType string `json:"contentType"`
}

// ListObjects - list objects api.
func (web *webAPIHandlers) ListObjects(r *http.Request, args *ListObjectsArgs, reply *ListObjectsRep) error {
	reply.UIVersion = browser.UIVersion
	objectAPI := web.ObjectAPI()
	if objectAPI == nil {
		return toJSONError(errServerNotInitialized)
	}

	listObjects := objectAPI.ListObjects
	if web.CacheAPI() != nil {
		listObjects = web.CacheAPI().ListObjects
	}

	claims, owner, authErr := webRequestAuthenticate(r)
	if authErr != nil {
		if authErr == errNoAuthToken {
			// Check if anonymous (non-owner) has access to download objects.
			readable := globalPolicySys.IsAllowed(policy.Args{
				Action:          policy.GetObjectAction,
				BucketName:      args.BucketName,
				ConditionValues: getConditionValues(r, ""),
				IsOwner:         false,
				ObjectName:      args.Prefix + "/",
			})

			// Check if anonymous (non-owner) has access to upload objects.
			writable := globalPolicySys.IsAllowed(policy.Args{
				Action:          policy.PutObjectAction,
				BucketName:      args.BucketName,
				ConditionValues: getConditionValues(r, ""),
				IsOwner:         false,
				ObjectName:      args.Prefix + "/",
			})

			reply.Writable = writable
			if !readable {
				// Error out if anonymous user (non-owner) has no access to download or upload objects
				if !writable {
					return errAccessDenied
				}
				// return empty object list if access is write only
				return nil
			}
		} else {
			return toJSONError(authErr)
		}
	}

	// For authenticated users apply IAM policy.
	if authErr == nil {
		readable := globalIAMSys.IsAllowed(iampolicy.Args{
			AccountName:     claims.Subject,
			Action:          iampolicy.Action(policy.GetObjectAction),
			BucketName:      args.BucketName,
			ConditionValues: getConditionValues(r, ""),
			IsOwner:         owner,
			ObjectName:      args.Prefix + "/",
		})

		writable := globalIAMSys.IsAllowed(iampolicy.Args{
			AccountName:     claims.Subject,
			Action:          iampolicy.Action(policy.PutObjectAction),
			BucketName:      args.BucketName,
			ConditionValues: getConditionValues(r, ""),
			IsOwner:         owner,
			ObjectName:      args.Prefix + "/",
		})

		reply.Writable = writable
		if !readable {
			// Error out if anonymous user (non-owner) has no access to download or upload objects
			if !writable {
				return errAccessDenied
			}
			// return empty object list if access is write only
			return nil
		}
	}

	lo, err := listObjects(context.Background(), args.BucketName, args.Prefix, args.Marker, slashSeparator, 1000)
	if err != nil {
		return &json2.Error{Message: err.Error()}
	}
	for i := range lo.Objects {
		if crypto.IsEncrypted(lo.Objects[i].UserDefined) {
			lo.Objects[i].Size, err = lo.Objects[i].DecryptedSize()
			if err != nil {
				return toJSONError(err)
			}
		}
	}
	reply.NextMarker = lo.NextMarker
	reply.IsTruncated = lo.IsTruncated
	for _, obj := range lo.Objects {
		reply.Objects = append(reply.Objects, WebObjectInfo{
			Key:          obj.Name,
			LastModified: obj.ModTime,
			Size:         obj.Size,
			ContentType:  obj.ContentType,
		})
	}
	for _, prefix := range lo.Prefixes {
		reply.Objects = append(reply.Objects, WebObjectInfo{
			Key: prefix,
		})
	}

	return nil
}

// RemoveObjectArgs - args to remove an object, JSON will look like.
//
// {
//     "bucketname": "testbucket",
//     "objects": [
//         "photos/hawaii/",
//         "photos/maldives/",
//         "photos/sanjose.jpg"
//     ]
// }
type RemoveObjectArgs struct {
	Objects    []string `json:"objects"`    // Contains objects, prefixes.
	BucketName string   `json:"bucketname"` // Contains bucket name.
}

// RemoveObject - removes an object, or all the objects at a given prefix.
func (web *webAPIHandlers) RemoveObject(r *http.Request, args *RemoveObjectArgs, reply *WebGenericRep) error {
	objectAPI := web.ObjectAPI()
	if objectAPI == nil {
		return toJSONError(errServerNotInitialized)
	}
	listObjects := objectAPI.ListObjects
	if web.CacheAPI() != nil {
		listObjects = web.CacheAPI().ListObjects
	}

	claims, owner, authErr := webRequestAuthenticate(r)
	if authErr != nil {
		return toJSONError(authErr)
	}

	if args.BucketName == "" || len(args.Objects) == 0 {
		return toJSONError(errInvalidArgument)
	}

	var err error
next:
	for _, objectName := range args.Objects {
		// If not a directory, remove the object.
		if !hasSuffix(objectName, slashSeparator) && objectName != "" {
			// Deny if WORM is enabled
			if globalWORMEnabled {
				if _, err = objectAPI.GetObjectInfo(context.Background(), args.BucketName, objectName, ObjectOptions{}); err == nil {
					return toJSONError(errMethodNotAllowed)
				}
			}

			if !globalIAMSys.IsAllowed(iampolicy.Args{
				AccountName:     claims.Subject,
				Action:          iampolicy.Action(policy.DeleteObjectAction),
				BucketName:      args.BucketName,
				ConditionValues: getConditionValues(r, ""),
				IsOwner:         owner,
				ObjectName:      objectName,
			}) {
				return toJSONError(errAccessDenied)
			}

			if err = deleteObject(nil, objectAPI, web.CacheAPI(), args.BucketName, objectName, r); err != nil {
				break next
			}
			continue
		}

		if !globalIAMSys.IsAllowed(iampolicy.Args{
			AccountName:     claims.Subject,
			Action:          iampolicy.Action(policy.DeleteObjectAction),
			BucketName:      args.BucketName,
			ConditionValues: getConditionValues(r, ""),
			IsOwner:         owner,
			ObjectName:      objectName,
		}) {
			return toJSONError(errAccessDenied)
		}

		// For directories, list the contents recursively and remove.
		marker := ""
		for {
			var lo ListObjectsInfo
			lo, err = listObjects(context.Background(), args.BucketName, objectName, marker, "", 1000)
			if err != nil {
				break next
			}
			marker = lo.NextMarker
			for _, obj := range lo.Objects {
				err = deleteObject(nil, objectAPI, web.CacheAPI(), args.BucketName, obj.Name, r)
				if err != nil {
					break next
				}
			}
			if !lo.IsTruncated {
				break
			}
		}
	}

	if err != nil && !isErrObjectNotFound(err) {
		// Ignore object not found error.
		return toJSONError(err, args.BucketName, "")
	}

	reply.UIVersion = browser.UIVersion
	return nil
}

// LoginArgs - login arguments.
type LoginArgs struct {
	Username string `json:"username" form:"username"`
	Password string `json:"password" form:"password"`
}

// LoginRep - login reply.
type LoginRep struct {
	Token     string `json:"token"`
	UIVersion string `json:"uiVersion"`
}

// Login - user login handler.
func (web *webAPIHandlers) Login(r *http.Request, args *LoginArgs, reply *LoginRep) error {
	token, err := authenticateWeb(args.Username, args.Password)
	if err != nil {
		return toJSONError(err)
	}

	reply.Token = token
	reply.UIVersion = browser.UIVersion
	return nil
}

// GenerateAuthReply - reply for GenerateAuth
type GenerateAuthReply struct {
	AccessKey string `json:"accessKey"`
	SecretKey string `json:"secretKey"`
	UIVersion string `json:"uiVersion"`
}

func (web webAPIHandlers) GenerateAuth(r *http.Request, args *WebGenericArgs, reply *GenerateAuthReply) error {
	_, owner, authErr := webRequestAuthenticate(r)
	if authErr != nil {
		return toJSONError(authErr)
	}
	if !owner {
		return toJSONError(errAccessDenied)
	}
	cred, err := auth.GetNewCredentials()
	if err != nil {
		return toJSONError(err)
	}
	reply.AccessKey = cred.AccessKey
	reply.SecretKey = cred.SecretKey
	reply.UIVersion = browser.UIVersion
	return nil
}

// SetAuthArgs - argument for SetAuth
type SetAuthArgs struct {
	AccessKey string `json:"accessKey"`
	SecretKey string `json:"secretKey"`
}

// SetAuthReply - reply for SetAuth
type SetAuthReply struct {
	Token       string            `json:"token"`
	UIVersion   string            `json:"uiVersion"`
	PeerErrMsgs map[string]string `json:"peerErrMsgs"`
}

// SetAuth - Set accessKey and secretKey credentials.
func (web *webAPIHandlers) SetAuth(r *http.Request, args *SetAuthArgs, reply *SetAuthReply) error {
	_, owner, authErr := webRequestAuthenticate(r)
	if authErr != nil {
		return toJSONError(authErr)
	}

	// If creds are set through ENV disallow changing credentials.
	// TODO: Multi-user credentials also cannot be changed from browser.
	if globalIsEnvCreds || globalWORMEnabled || !owner {
		return toJSONError(errChangeCredNotAllowed)
	}

	creds, err := auth.CreateCredentials(args.AccessKey, args.SecretKey)
	if err != nil {
		return toJSONError(err)
	}

	// Acquire lock before updating global configuration.
	globalServerConfigMu.Lock()
	defer globalServerConfigMu.Unlock()

	// Update credentials in memory
	prevCred := globalServerConfig.SetCredential(creds)

	// Persist updated credentials.
	if err = saveServerConfig(context.Background(), newObjectLayerFn(), globalServerConfig); err != nil {
		// Save the current creds when failed to update.
		globalServerConfig.SetCredential(prevCred)
		logger.LogIf(context.Background(), err)
		return toJSONError(err)
	}

	if errs := globalNotificationSys.LoadCredentials(); len(errs) != 0 {
		reply.PeerErrMsgs = make(map[string]string)
		for host, err := range errs {
			err = fmt.Errorf("Unable to update credentials on server %v: %v", host, err)
			logger.LogIf(context.Background(), err)
			reply.PeerErrMsgs[host.String()] = err.Error()
		}
	} else {
		reply.Token, err = authenticateWeb(creds.AccessKey, creds.SecretKey)
		if err != nil {
			return toJSONError(err)
		}
		reply.UIVersion = browser.UIVersion
	}

	return nil
}

// GetAuthReply - Reply current credentials.
type GetAuthReply struct {
	AccessKey string `json:"accessKey"`
	SecretKey string `json:"secretKey"`
	UIVersion string `json:"uiVersion"`
}

// GetAuth - return accessKey and secretKey credentials.
func (web *webAPIHandlers) GetAuth(r *http.Request, args *WebGenericArgs, reply *GetAuthReply) error {
	_, owner, authErr := webRequestAuthenticate(r)
	if authErr != nil {
		return toJSONError(authErr)
	}
	if !owner {
		return toJSONError(errAccessDenied)
	}
	creds := globalServerConfig.GetCredential()
	reply.AccessKey = creds.AccessKey
	reply.SecretKey = creds.SecretKey
	reply.UIVersion = browser.UIVersion
	return nil
}

// URLTokenReply contains the reply for CreateURLToken.
type URLTokenReply struct {
	Token     string `json:"token"`
	UIVersion string `json:"uiVersion"`
}

// CreateURLToken creates a URL token (short-lived) for GET requests.
func (web *webAPIHandlers) CreateURLToken(r *http.Request, args *WebGenericArgs, reply *URLTokenReply) error {
	claims, owner, authErr := webRequestAuthenticate(r)
	if authErr != nil {
		return toJSONError(authErr)
	}

	creds := globalServerConfig.GetCredential()
	if !owner {
		var ok bool
		creds, ok = globalIAMSys.GetUser(claims.Subject)
		if !ok {
			return toJSONError(errInvalidAccessKeyID)
		}
	}

	token, err := authenticateURL(creds.AccessKey, creds.SecretKey)
	if err != nil {
		return toJSONError(err)
	}

	reply.Token = token
	reply.UIVersion = browser.UIVersion
	return nil
}

// Upload - file upload handler.
func (web *webAPIHandlers) Upload(w http.ResponseWriter, r *http.Request) {
	ctx := newContext(r, w, "WebUpload")

	defer logger.AuditLog(ctx, w, r)

	objectAPI := web.ObjectAPI()
	if objectAPI == nil {
		writeWebErrorResponse(w, errServerNotInitialized)
		return
	}

	putObject := objectAPI.PutObject
	if web.CacheAPI() != nil {
		putObject = web.CacheAPI().PutObject
	}
	vars := mux.Vars(r)
	bucket := vars["bucket"]
	object := vars["object"]

	claims, owner, authErr := webRequestAuthenticate(r)
	if authErr != nil {
		if authErr == errNoAuthToken {
			// Check if anonymous (non-owner) has access to upload objects.
			if !globalPolicySys.IsAllowed(policy.Args{
				Action:          policy.PutObjectAction,
				BucketName:      bucket,
				ConditionValues: getConditionValues(r, ""),
				IsOwner:         false,
				ObjectName:      object,
			}) {
				writeWebErrorResponse(w, errAuthentication)
				return
			}
		} else {
			writeWebErrorResponse(w, authErr)
			return
		}
	}

	// For authenticated users apply IAM policy.
	if authErr == nil {
		if !globalIAMSys.IsAllowed(iampolicy.Args{
			AccountName:     claims.Subject,
			Action:          iampolicy.Action(policy.PutObjectAction),
			BucketName:      bucket,
			ConditionValues: getConditionValues(r, ""),
			IsOwner:         owner,
			ObjectName:      object,
		}) {
			writeWebErrorResponse(w, errAuthentication)
			return
		}
	}

	// Require Content-Length to be set in the request
	size := r.ContentLength
	if size < 0 {
		writeWebErrorResponse(w, errSizeUnspecified)
		return
	}

	// Extract incoming metadata if any.
	metadata, err := extractMetadata(context.Background(), r)
	if err != nil {
		writeErrorResponse(w, ErrInternalError, r.URL)
		return
	}

	reader := r.Body
	actualSize := size

	if objectAPI.IsCompressionSupported() && isCompressible(r.Header, object) && size > 0 {
		// Storing the compression metadata.
		metadata[ReservedMetadataPrefix+"compression"] = compressionAlgorithmV1
		metadata[ReservedMetadataPrefix+"actual-size"] = strconv.FormatInt(size, 10)

		pipeReader, pipeWriter := io.Pipe()
		snappyWriter := snappy.NewWriter(pipeWriter)

		var actualReader *hash.Reader
		actualReader, err = hash.NewReader(reader, size, "", "", actualSize)
		if err != nil {
			writeWebErrorResponse(w, err)
			return
		}

		go func() {
			// Writing to the compressed writer.
			_, cerr := io.CopyN(snappyWriter, actualReader, actualSize)
			snappyWriter.Close()
			pipeWriter.CloseWithError(cerr)
		}()

		// Set compression metrics.
		size = -1 // Since compressed size is un-predictable.
		reader = pipeReader
	}

	hashReader, err := hash.NewReader(reader, size, "", "", actualSize)
	if err != nil {
		writeWebErrorResponse(w, err)
		return
	}
	opts := ObjectOptions{}
	// Deny if WORM is enabled
	if globalWORMEnabled {
		if _, err = objectAPI.GetObjectInfo(ctx, bucket, object, opts); err == nil {
			writeWebErrorResponse(w, errMethodNotAllowed)
			return
		}
	}

	objInfo, err := putObject(ctx, bucket, object, hashReader, metadata, opts)
	if err != nil {
		writeWebErrorResponse(w, err)
		return
	}

	// Get host and port from Request.RemoteAddr.
	host, port, err := net.SplitHostPort(handlers.GetSourceIP(r))
	if err != nil {
		host, port = "", ""
	}

	// Notify object created event.
	sendEvent(eventArgs{
		EventName:    event.ObjectCreatedPut,
		BucketName:   bucket,
		Object:       objInfo,
		ReqParams:    extractReqParams(r),
		RespElements: extractRespElements(w),
		UserAgent:    r.UserAgent(),
		Host:         host,
		Port:         port,
	})
}

// Download - file download handler.
func (web *webAPIHandlers) Download(w http.ResponseWriter, r *http.Request) {
	ctx := newContext(r, w, "WebDownload")

	defer logger.AuditLog(ctx, w, r)

	var wg sync.WaitGroup
	objectAPI := web.ObjectAPI()
	if objectAPI == nil {
		writeWebErrorResponse(w, errServerNotInitialized)
		return
	}

	vars := mux.Vars(r)
	bucket := vars["bucket"]
	object := vars["object"]
	token := r.URL.Query().Get("token")

	claims, owner, authErr := webTokenAuthenticate(token)
	if authErr != nil {
		if authErr == errNoAuthToken {
			// Check if anonymous (non-owner) has access to download objects.
			if !globalPolicySys.IsAllowed(policy.Args{
				Action:          policy.GetObjectAction,
				BucketName:      bucket,
				ConditionValues: getConditionValues(r, ""),
				IsOwner:         false,
				ObjectName:      object,
			}) {
				writeWebErrorResponse(w, errAuthentication)
				return
			}
		} else {
			writeWebErrorResponse(w, authErr)
			return
		}
	}

	// For authenticated users apply IAM policy.
	if authErr == nil {
		if !globalIAMSys.IsAllowed(iampolicy.Args{
			AccountName:     claims.Subject,
			Action:          iampolicy.Action(policy.GetObjectAction),
			BucketName:      bucket,
			ConditionValues: getConditionValues(r, ""),
			IsOwner:         owner,
			ObjectName:      object,
		}) {
			writeWebErrorResponse(w, errAuthentication)
			return
		}
	}

	opts := ObjectOptions{}
	getObjectInfo := objectAPI.GetObjectInfo
	getObject := objectAPI.GetObject
	if web.CacheAPI() != nil {
		getObjectInfo = web.CacheAPI().GetObjectInfo
		getObject = web.CacheAPI().GetObject
	}
	objInfo, err := getObjectInfo(ctx, bucket, object, opts)
	if err != nil {
		writeWebErrorResponse(w, err)
		return
	}
	length := objInfo.Size
	var actualSize int64
	if objInfo.IsCompressed() {
		// Read the decompressed size from the meta.json.
		actualSize = objInfo.GetActualSize()
		if actualSize < 0 {
			return
		}
	}
	if objectAPI.IsEncryptionSupported() {
		if _, err = DecryptObjectInfo(objInfo, r.Header); err != nil {
			writeWebErrorResponse(w, err)
			return
		}
		if crypto.IsEncrypted(objInfo.UserDefined) {
			length, _ = objInfo.DecryptedSize()
		}
	}
	var startOffset int64
	var writer io.Writer
	if objInfo.IsCompressed() {
		// The decompress metrics are set.
		snappyStartOffset := 0
		snappyLength := actualSize

		// Open a pipe for compression
		// Where compressWriter is actually passed to the getObject
		decompressReader, compressWriter := io.Pipe()
		snappyReader := snappy.NewReader(decompressReader)

		// The limit is set to the actual size.
		responseWriter := ioutil.LimitedWriter(w, int64(snappyStartOffset), snappyLength)
		wg.Add(1) //For closures.
		go func() {
			defer wg.Done()

			// Finally, writes to the client.
			_, perr := io.Copy(responseWriter, snappyReader)

			// Close the compressWriter if the data is read already.
			// Closing the pipe, releases the writer passed to the getObject.
			compressWriter.CloseWithError(perr)
		}()
		writer = compressWriter
	} else {
		writer = w
	}
	if objectAPI.IsEncryptionSupported() && crypto.S3.IsEncrypted(objInfo.UserDefined) {
		// Response writer should be limited early on for decryption upto required length,
		// additionally also skipping mod(offset)64KiB boundaries.
		writer = ioutil.LimitedWriter(writer, startOffset%(64*1024), length)

		writer, startOffset, length, err = DecryptBlocksRequest(writer, r, bucket, object, startOffset, length, objInfo, false)
		if err != nil {
			writeWebErrorResponse(w, err)
			return
		}
		w.Header().Set(crypto.SSEHeader, crypto.SSEAlgorithmAES256)
	}

	httpWriter := ioutil.WriteOnClose(writer)

	// Add content disposition.
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", path.Base(object)))

	if err = getObject(ctx, bucket, object, 0, -1, httpWriter, "", opts); err != nil {
		httpWriter.Close()
		if objInfo.IsCompressed() {
			wg.Wait()
		}
		/// No need to print error, response writer already written to.
		return
	}
	if err = httpWriter.Close(); err != nil {
		if !httpWriter.HasWritten() { // write error response only if no data has been written to client yet
			writeWebErrorResponse(w, err)
			return
		}
	}
	if objInfo.IsCompressed() {
		// Wait for decompression go-routine to retire.
		wg.Wait()
	}

	// Get host and port from Request.RemoteAddr.
	host, port, err := net.SplitHostPort(handlers.GetSourceIP(r))
	if err != nil {
		host, port = "", ""
	}

	// Notify object accessed via a GET request.
	sendEvent(eventArgs{
		EventName:    event.ObjectAccessedGet,
		BucketName:   bucket,
		Object:       objInfo,
		ReqParams:    extractReqParams(r),
		RespElements: extractRespElements(w),
		UserAgent:    r.UserAgent(),
		Host:         host,
		Port:         port,
	})
}

// DownloadZipArgs - Argument for downloading a bunch of files as a zip file.
// JSON will look like:
// '{"bucketname":"testbucket","prefix":"john/pics/","objects":["hawaii/","maldives/","sanjose.jpg"]}'
type DownloadZipArgs struct {
	Objects    []string `json:"objects"`    // can be files or sub-directories
	Prefix     string   `json:"prefix"`     // current directory in the browser-ui
	BucketName string   `json:"bucketname"` // bucket name.
}

// Takes a list of objects and creates a zip file that sent as the response body.
func (web *webAPIHandlers) DownloadZip(w http.ResponseWriter, r *http.Request) {
	// Get host and port from Request.RemoteAddr.
	host, port, err := net.SplitHostPort(handlers.GetSourceIP(r))
	if err != nil {
		host, port = "", ""
	}

	ctx := newContext(r, w, "WebDownloadZip")
	defer logger.AuditLog(ctx, w, r)

	var wg sync.WaitGroup
	objectAPI := web.ObjectAPI()
	if objectAPI == nil {
		writeWebErrorResponse(w, errServerNotInitialized)
		return
	}

	// Auth is done after reading the body to accommodate for anonymous requests
	// when bucket policy is enabled.
	var args DownloadZipArgs
	tenKB := 10 * 1024 // To limit r.Body to take care of misbehaving anonymous client.
	decodeErr := json.NewDecoder(io.LimitReader(r.Body, int64(tenKB))).Decode(&args)
	if decodeErr != nil {
		writeWebErrorResponse(w, decodeErr)
		return
	}

	token := r.URL.Query().Get("token")
	claims, owner, authErr := webTokenAuthenticate(token)
	if authErr != nil {
		if authErr == errNoAuthToken {
			for _, object := range args.Objects {
				// Check if anonymous (non-owner) has access to download objects.
				if !globalPolicySys.IsAllowed(policy.Args{
					Action:          policy.GetObjectAction,
					BucketName:      args.BucketName,
					ConditionValues: getConditionValues(r, ""),
					IsOwner:         false,
					ObjectName:      pathJoin(args.Prefix, object),
				}) {
					writeWebErrorResponse(w, errAuthentication)
					return
				}
			}
		} else {
			writeWebErrorResponse(w, authErr)
			return
		}
	}

	// For authenticated users apply IAM policy.
	if authErr == nil {
		for _, object := range args.Objects {
			if !globalIAMSys.IsAllowed(iampolicy.Args{
				AccountName:     claims.Subject,
				Action:          iampolicy.Action(policy.GetObjectAction),
				BucketName:      args.BucketName,
				ConditionValues: getConditionValues(r, ""),
				IsOwner:         owner,
				ObjectName:      pathJoin(args.Prefix, object),
			}) {
				writeWebErrorResponse(w, errAuthentication)
				return
			}
		}
	}

	getObject := objectAPI.GetObject
	if web.CacheAPI() != nil {
		getObject = web.CacheAPI().GetObject
	}
	listObjects := objectAPI.ListObjects
	if web.CacheAPI() != nil {
		listObjects = web.CacheAPI().ListObjects
	}

	archive := zip.NewWriter(w)
	defer archive.Close()

	getObjectInfo := objectAPI.GetObjectInfo
	if web.CacheAPI() != nil {
		getObjectInfo = web.CacheAPI().GetObjectInfo
	}
	opts := ObjectOptions{}
	var length int64
	for _, object := range args.Objects {
		// Writes compressed object file to the response.
		zipit := func(objectName string) error {
			info, err := getObjectInfo(ctx, args.BucketName, objectName, opts)
			if err != nil {
				return err
			}
			length = info.Size
			if objectAPI.IsEncryptionSupported() {
				if _, err = DecryptObjectInfo(info, r.Header); err != nil {
					writeWebErrorResponse(w, err)
					return err
				}
				if crypto.IsEncrypted(info.UserDefined) {
					length, _ = info.DecryptedSize()
				}
			}
			length = info.Size
			var actualSize int64
			if info.IsCompressed() {
				// Read the decompressed size from the meta.json.
				actualSize = info.GetActualSize()
				// Set the info.Size to the actualSize.
				info.Size = actualSize
			}
			header := &zip.FileHeader{
				Name:               strings.TrimPrefix(objectName, args.Prefix),
				Method:             zip.Deflate,
				UncompressedSize64: uint64(length),
				UncompressedSize:   uint32(length),
			}
			zipWriter, err := archive.CreateHeader(header)
			if err != nil {
				writeWebErrorResponse(w, errUnexpected)
				return err
			}
			var startOffset int64
			var writer io.Writer

			if info.IsCompressed() {
				// The decompress metrics are set.
				snappyStartOffset := 0
				snappyLength := actualSize

				// Open a pipe for compression
				// Where compressWriter is actually passed to the getObject
				decompressReader, compressWriter := io.Pipe()
				snappyReader := snappy.NewReader(decompressReader)

				// The limit is set to the actual size.
				responseWriter := ioutil.LimitedWriter(zipWriter, int64(snappyStartOffset), snappyLength)
				wg.Add(1) //For closures.
				go func() {
					defer wg.Done()
					// Finally, writes to the client.
					_, perr := io.Copy(responseWriter, snappyReader)

					// Close the compressWriter if the data is read already.
					// Closing the pipe, releases the writer passed to the getObject.
					compressWriter.CloseWithError(perr)
				}()
				writer = compressWriter
			} else {
				writer = zipWriter
			}
			if objectAPI.IsEncryptionSupported() && crypto.S3.IsEncrypted(info.UserDefined) {
				// Response writer should be limited early on for decryption upto required length,
				// additionally also skipping mod(offset)64KiB boundaries.
				writer = ioutil.LimitedWriter(writer, startOffset%(64*1024), length)
				writer, startOffset, length, err = DecryptBlocksRequest(writer, r, args.BucketName, objectName, startOffset, length, info, false)
				if err != nil {
					writeWebErrorResponse(w, err)
					return err
				}
			}
			httpWriter := ioutil.WriteOnClose(writer)
			if err = getObject(ctx, args.BucketName, objectName, 0, length, httpWriter, "", opts); err != nil {
				httpWriter.Close()
				if info.IsCompressed() {
					// Wait for decompression go-routine to retire.
					wg.Wait()
				}
				return err
			}
			if err = httpWriter.Close(); err != nil {
				if !httpWriter.HasWritten() { // write error response only if no data has been written to client yet
					writeWebErrorResponse(w, err)
					return err
				}
			}
			if info.IsCompressed() {
				// Wait for decompression go-routine to retire.
				wg.Wait()
			}

			// Notify object accessed via a GET request.
			sendEvent(eventArgs{
				EventName:    event.ObjectAccessedGet,
				BucketName:   args.BucketName,
				Object:       info,
				ReqParams:    extractReqParams(r),
				RespElements: extractRespElements(w),
				UserAgent:    r.UserAgent(),
				Host:         host,
				Port:         port,
			})

			return nil
		}

		if !hasSuffix(object, slashSeparator) {
			// If not a directory, compress the file and write it to response.
			err := zipit(pathJoin(args.Prefix, object))
			if err != nil {
				return
			}
			continue
		}

		// For directories, list the contents recursively and write the objects as compressed
		// date to the response writer.
		marker := ""
		for {
			lo, err := listObjects(context.Background(), args.BucketName, pathJoin(args.Prefix, object), marker, "", 1000)
			if err != nil {
				return
			}
			marker = lo.NextMarker
			for _, obj := range lo.Objects {
				err = zipit(obj.Name)
				if err != nil {
					return
				}
			}
			if !lo.IsTruncated {
				break
			}
		}
	}
}

// GetBucketPolicyArgs - get bucket policy args.
type GetBucketPolicyArgs struct {
	BucketName string `json:"bucketName"`
	Prefix     string `json:"prefix"`
}

// GetBucketPolicyRep - get bucket policy reply.
type GetBucketPolicyRep struct {
	UIVersion string                     `json:"uiVersion"`
	Policy    miniogopolicy.BucketPolicy `json:"policy"`
}

// GetBucketPolicy - get bucket policy for the requested prefix.
func (web *webAPIHandlers) GetBucketPolicy(r *http.Request, args *GetBucketPolicyArgs, reply *GetBucketPolicyRep) error {
	objectAPI := web.ObjectAPI()
	if objectAPI == nil {
		return toJSONError(errServerNotInitialized)
	}

	_, owner, authErr := webRequestAuthenticate(r)
	if authErr != nil {
		return toJSONError(authErr)
	}
	if !owner {
		return toJSONError(errAccessDenied)
	}

	bucketPolicy, err := objectAPI.GetBucketPolicy(context.Background(), args.BucketName)
	if err != nil {
		if _, ok := err.(BucketPolicyNotFound); !ok {
			return toJSONError(err, args.BucketName)
		}
		return err
	}

	policyInfo, err := PolicyToBucketAccessPolicy(bucketPolicy)
	if err != nil {
		// This should not happen.
		return toJSONError(err, args.BucketName)
	}

	reply.UIVersion = browser.UIVersion
	reply.Policy = miniogopolicy.GetPolicy(policyInfo.Statements, args.BucketName, args.Prefix)

	return nil
}

// ListAllBucketPoliciesArgs - get all bucket policies.
type ListAllBucketPoliciesArgs struct {
	BucketName string `json:"bucketName"`
}

// BucketAccessPolicy - Collection of canned bucket policy at a given prefix.
type BucketAccessPolicy struct {
	Bucket string                     `json:"bucket"`
	Prefix string                     `json:"prefix"`
	Policy miniogopolicy.BucketPolicy `json:"policy"`
}

// ListAllBucketPoliciesRep - get all bucket policy reply.
type ListAllBucketPoliciesRep struct {
	UIVersion string               `json:"uiVersion"`
	Policies  []BucketAccessPolicy `json:"policies"`
}

// ListAllBucketPolicies - get all bucket policy.
func (web *webAPIHandlers) ListAllBucketPolicies(r *http.Request, args *ListAllBucketPoliciesArgs, reply *ListAllBucketPoliciesRep) error {
	objectAPI := web.ObjectAPI()
	if objectAPI == nil {
		return toJSONError(errServerNotInitialized)
	}

	_, owner, authErr := webRequestAuthenticate(r)
	if authErr != nil {
		return toJSONError(authErr)
	}
	if !owner {
		return toJSONError(errAccessDenied)
	}

	bucketPolicy, err := objectAPI.GetBucketPolicy(context.Background(), args.BucketName)
	if err != nil {
		if _, ok := err.(BucketPolicyNotFound); !ok {
			return toJSONError(err, args.BucketName)
		}
	}

	policyInfo, err := PolicyToBucketAccessPolicy(bucketPolicy)
	if err != nil {
		// This should not happen.
		return toJSONError(err, args.BucketName)
	}

	reply.UIVersion = browser.UIVersion
	for prefix, policy := range miniogopolicy.GetPolicies(policyInfo.Statements, args.BucketName, "") {
		bucketName, objectPrefix := urlPath2BucketObjectName(prefix)
		objectPrefix = strings.TrimSuffix(objectPrefix, "*")
		reply.Policies = append(reply.Policies, BucketAccessPolicy{
			Bucket: bucketName,
			Prefix: objectPrefix,
			Policy: policy,
		})
	}

	return nil
}

// SetBucketPolicyWebArgs - set bucket policy args.
type SetBucketPolicyWebArgs struct {
	BucketName string `json:"bucketName"`
	Prefix     string `json:"prefix"`
	Policy     string `json:"policy"`
}

// SetBucketPolicy - set bucket policy.
func (web *webAPIHandlers) SetBucketPolicy(r *http.Request, args *SetBucketPolicyWebArgs, reply *WebGenericRep) error {
	objectAPI := web.ObjectAPI()
	reply.UIVersion = browser.UIVersion

	if objectAPI == nil {
		return toJSONError(errServerNotInitialized)
	}

	_, owner, authErr := webRequestAuthenticate(r)
	if authErr != nil {
		return toJSONError(authErr)
	}
	if !owner {
		return toJSONError(errAccessDenied)
	}

	policyType := miniogopolicy.BucketPolicy(args.Policy)
	if !policyType.IsValidBucketPolicy() {
		return &json2.Error{
			Message: "Invalid policy type " + args.Policy,
		}
	}

	ctx := context.Background()

	bucketPolicy, err := objectAPI.GetBucketPolicy(ctx, args.BucketName)
	if err != nil {
		if _, ok := err.(BucketPolicyNotFound); !ok {
			return toJSONError(err, args.BucketName)
		}
	}

	policyInfo, err := PolicyToBucketAccessPolicy(bucketPolicy)
	if err != nil {
		// This should not happen.
		return toJSONError(err, args.BucketName)
	}

	policyInfo.Statements = miniogopolicy.SetPolicy(policyInfo.Statements, policyType, args.BucketName, args.Prefix)

	if len(policyInfo.Statements) == 0 {
		if err = objectAPI.DeleteBucketPolicy(ctx, args.BucketName); err != nil {
			return toJSONError(err, args.BucketName)
		}

		globalPolicySys.Remove(args.BucketName)
		return nil
	}

	bucketPolicy, err = BucketAccessPolicyToPolicy(policyInfo)
	if err != nil {
		// This should not happen.
		return toJSONError(err, args.BucketName)
	}

	// Parse validate and save bucket policy.
	if err := objectAPI.SetBucketPolicy(ctx, args.BucketName, bucketPolicy); err != nil {
		return toJSONError(err, args.BucketName)
	}

	globalPolicySys.Set(args.BucketName, *bucketPolicy)
	globalNotificationSys.SetBucketPolicy(ctx, args.BucketName, bucketPolicy)

	return nil
}

// PresignedGetArgs - presigned-get API args.
type PresignedGetArgs struct {
	// Host header required for signed headers.
	HostName string `json:"host"`

	// Bucket name of the object to be presigned.
	BucketName string `json:"bucket"`

	// Object name to be presigned.
	ObjectName string `json:"object"`

	// Expiry in seconds.
	Expiry int64 `json:"expiry"`
}

// PresignedGetRep - presigned-get URL reply.
type PresignedGetRep struct {
	UIVersion string `json:"uiVersion"`
	// Presigned URL of the object.
	URL string `json:"url"`
}

// PresignedGET - returns presigned-Get url.
func (web *webAPIHandlers) PresignedGet(r *http.Request, args *PresignedGetArgs, reply *PresignedGetRep) error {
	claims, owner, authErr := webRequestAuthenticate(r)
	if authErr != nil {
		return toJSONError(authErr)
	}
	var creds auth.Credentials
	if !owner {
		var ok bool
		creds, ok = globalIAMSys.GetUser(claims.Subject)
		if !ok {
			return toJSONError(errInvalidAccessKeyID)
		}
	} else {
		creds = globalServerConfig.GetCredential()
	}

	region := globalServerConfig.GetRegion()
	if args.BucketName == "" || args.ObjectName == "" {
		return &json2.Error{
			Message: "Bucket and Object are mandatory arguments.",
		}
	}

	reply.UIVersion = browser.UIVersion
	reply.URL = presignedGet(args.HostName, args.BucketName, args.ObjectName, args.Expiry, creds, region)
	return nil
}

// Returns presigned url for GET method.
func presignedGet(host, bucket, object string, expiry int64, creds auth.Credentials, region string) string {
	accessKey := creds.AccessKey
	secretKey := creds.SecretKey

	date := UTCNow()
	dateStr := date.Format(iso8601Format)
	credential := fmt.Sprintf("%s/%s", accessKey, getScope(date, region))

	var expiryStr = "604800" // Default set to be expire in 7days.
	if expiry < 604800 && expiry > 0 {
		expiryStr = strconv.FormatInt(expiry, 10)
	}

	query := url.Values{}
	query.Set("X-Amz-Algorithm", signV4Algorithm)
	query.Set("X-Amz-Credential", credential)
	query.Set("X-Amz-Date", dateStr)
	query.Set("X-Amz-Expires", expiryStr)
	query.Set("X-Amz-SignedHeaders", "host")
	queryStr := s3utils.QueryEncode(query)

	path := "/" + path.Join(bucket, object)

	// "host" is the only header required to be signed for Presigned URLs.
	extractedSignedHeaders := make(http.Header)
	extractedSignedHeaders.Set("host", host)
	canonicalRequest := getCanonicalRequest(extractedSignedHeaders, unsignedPayload, queryStr, path, "GET")
	stringToSign := getStringToSign(canonicalRequest, date, getScope(date, region))
	signingKey := getSigningKey(secretKey, date, region)
	signature := getSignature(signingKey, stringToSign)

	// Construct the final presigned URL.
	return host + s3utils.EncodePath(path) + "?" + queryStr + "&" + "X-Amz-Signature=" + signature
}

// toJSONError converts regular errors into more user friendly
// and consumable error message for the browser UI.
func toJSONError(err error, params ...string) (jerr *json2.Error) {
	apiErr := toWebAPIError(err)
	jerr = &json2.Error{
		Message: apiErr.Description,
	}
	switch apiErr.Code {
	// Reserved bucket name provided.
	case "AllAccessDisabled":
		if len(params) > 0 {
			jerr = &json2.Error{
				Message: fmt.Sprintf("All access to this bucket %s has been disabled.", params[0]),
			}
		}
	// Bucket name invalid with custom error message.
	case "InvalidBucketName":
		if len(params) > 0 {
			jerr = &json2.Error{
				Message: fmt.Sprintf("Bucket Name %s is invalid. Lowercase letters, period, hyphen, numerals are the only allowed characters and should be minimum 3 characters in length.", params[0]),
			}
		}
	// Bucket not found custom error message.
	case "NoSuchBucket":
		if len(params) > 0 {
			jerr = &json2.Error{
				Message: fmt.Sprintf("The specified bucket %s does not exist.", params[0]),
			}
		}
	// Object not found custom error message.
	case "NoSuchKey":
		if len(params) > 1 {
			jerr = &json2.Error{
				Message: fmt.Sprintf("The specified key %s does not exist", params[1]),
			}
		}
		// Add more custom error messages here with more context.
	}
	return jerr
}

// toWebAPIError - convert into error into APIError.
func toWebAPIError(err error) APIError {
	if err == errAuthentication {
		return APIError{
			Code:           "AccessDenied",
			HTTPStatusCode: http.StatusForbidden,
			Description:    err.Error(),
		}
	} else if err == errServerNotInitialized {
		return APIError{
			Code:           "XMinioServerNotInitialized",
			HTTPStatusCode: http.StatusServiceUnavailable,
			Description:    err.Error(),
		}
	} else if err == auth.ErrInvalidAccessKeyLength {
		return APIError{
			Code:           "AccessDenied",
			HTTPStatusCode: http.StatusForbidden,
			Description:    err.Error(),
		}
	} else if err == auth.ErrInvalidSecretKeyLength {
		return APIError{
			Code:           "AccessDenied",
			HTTPStatusCode: http.StatusForbidden,
			Description:    err.Error(),
		}
	} else if err == errInvalidAccessKeyID {
		return APIError{
			Code:           "AccessDenied",
			HTTPStatusCode: http.StatusForbidden,
			Description:    err.Error(),
		}
	} else if err == errSizeUnspecified {
		return APIError{
			Code:           "InvalidRequest",
			HTTPStatusCode: http.StatusBadRequest,
			Description:    err.Error(),
		}
	} else if err == errChangeCredNotAllowed {
		return APIError{
			Code:           "MethodNotAllowed",
			HTTPStatusCode: http.StatusMethodNotAllowed,
			Description:    err.Error(),
		}
	} else if err == errInvalidBucketName {
		return APIError{
			Code:           "InvalidBucketName",
			HTTPStatusCode: http.StatusBadRequest,
			Description:    err.Error(),
		}
	} else if err == errInvalidArgument {
		return APIError{
			Code:           "InvalidArgument",
			HTTPStatusCode: http.StatusBadRequest,
			Description:    err.Error(),
		}
	} else if err == errEncryptedObject {
		return getAPIError(ErrSSEEncryptedObject)
	} else if err == errInvalidEncryptionParameters {
		return getAPIError(ErrInvalidEncryptionParameters)
	} else if err == errObjectTampered {
		return getAPIError(ErrObjectTampered)
	} else if err == errMethodNotAllowed {
		return getAPIError(ErrMethodNotAllowed)
	}

	// Convert error type to api error code.
	switch err.(type) {
	case StorageFull:
		return getAPIError(ErrStorageFull)
	case BucketNotFound:
		return getAPIError(ErrNoSuchBucket)
	case BucketExists:
		return getAPIError(ErrBucketAlreadyOwnedByYou)
	case BucketNameInvalid:
		return getAPIError(ErrInvalidBucketName)
	case hash.BadDigest:
		return getAPIError(ErrBadDigest)
	case IncompleteBody:
		return getAPIError(ErrIncompleteBody)
	case ObjectExistsAsDirectory:
		return getAPIError(ErrObjectExistsAsDirectory)
	case ObjectNotFound:
		return getAPIError(ErrNoSuchKey)
	case ObjectNameInvalid:
		return getAPIError(ErrNoSuchKey)
	case InsufficientWriteQuorum:
		return getAPIError(ErrWriteQuorum)
	case InsufficientReadQuorum:
		return getAPIError(ErrReadQuorum)
	case PolicyNesting:
		return getAPIError(ErrPolicyNesting)
	case NotImplemented:
		return APIError{
			Code:           "NotImplemented",
			HTTPStatusCode: http.StatusBadRequest,
			Description:    "Functionality not implemented",
		}
	}

	// Log unexpected and unhandled errors.
	logger.LogIf(context.Background(), err)
	return APIError{
		Code:           "InternalError",
		HTTPStatusCode: http.StatusInternalServerError,
		Description:    err.Error(),
	}
}

// writeWebErrorResponse - set HTTP status code and write error description to the body.
func writeWebErrorResponse(w http.ResponseWriter, err error) {
	apiErr := toWebAPIError(err)
	w.WriteHeader(apiErr.HTTPStatusCode)
	w.Write([]byte(apiErr.Description))
}
