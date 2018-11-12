/*
 * Minio Cloud Storage, (C) 2017, 2018 Minio, Inc.
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

package sia

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	humanize "github.com/dustin/go-humanize"
	"github.com/fatih/color"
	"github.com/minio/cli"
	"github.com/minio/minio-go/pkg/set"
	minio "github.com/minio/minio/cmd"
	"github.com/minio/minio/cmd/logger"
	"github.com/minio/minio/pkg/auth"
	"github.com/minio/minio/pkg/hash"
	"github.com/minio/sha256-simd"
)

const (
	siaBackend = "sia"
)

type siaObjects struct {
	minio.GatewayUnsupported
	Address  string // Address and port of Sia Daemon.
	TempDir  string // Temporary storage location for file transfers.
	RootDir  string // Root directory to store files on Sia.
	password string // Sia password for uploading content in authenticated manner.
}

func init() {
	const siaGatewayTemplate = `NAME:
  {{.HelpName}} - {{.Usage}}

USAGE:
  {{.HelpName}} {{if .VisibleFlags}}[FLAGS]{{end}} [SIA_DAEMON_ADDR]
{{if .VisibleFlags}}
FLAGS:
  {{range .VisibleFlags}}{{.}}
  {{end}}{{end}}
ENVIRONMENT VARIABLES: (Default values in parenthesis)
  ACCESS:
     MINIO_ACCESS_KEY: Custom access key (Do not reuse same access keys on all instances)
     MINIO_SECRET_KEY: Custom secret key (Do not reuse same secret keys on all instances)

  BROWSER:
     MINIO_BROWSER: To disable web browser access, set this value to "off".

  DOMAIN:
     MINIO_DOMAIN: To enable virtual-host-style requests, set this value to Minio host domain name.

  CACHE:
     MINIO_CACHE_DRIVES: List of mounted drives or directories delimited by ";".
     MINIO_CACHE_EXCLUDE: List of cache exclusion patterns delimited by ";".
     MINIO_CACHE_EXPIRY: Cache expiry duration in days.
     MINIO_CACHE_MAXUSE: Maximum permitted usage of the cache in percentage (0-100).

  SIA_TEMP_DIR:        The name of the local Sia temporary storage directory. (.sia_temp)
  SIA_API_PASSWORD:    API password for Sia daemon. (default is empty)

EXAMPLES:
  1. Start minio gateway server for Sia backend.
     $ {{.HelpName}}

  2. Start minio gateway server for Sia backend with edge caching enabled.
     $ export MINIO_CACHE_DRIVES="/mnt/drive1;/mnt/drive2;/mnt/drive3;/mnt/drive4"
     $ export MINIO_CACHE_EXCLUDE="bucket1/*;*.png"
     $ export MINIO_CACHE_EXPIRY=40
     $ export MINIO_CACHE_MAXUSE=80
     $ {{.HelpName}}
`

	minio.RegisterGatewayCommand(cli.Command{
		Name:               siaBackend,
		Usage:              "Sia Decentralized Cloud.",
		Action:             siaGatewayMain,
		CustomHelpTemplate: siaGatewayTemplate,
		HideHelpCommand:    true,
	})
}

// Handler for 'minio gateway sia' command line.
func siaGatewayMain(ctx *cli.Context) {
	// Validate gateway arguments.
	host := ctx.Args().First()
	// Validate gateway arguments.
	logger.FatalIf(minio.ValidateGatewayArguments(ctx.GlobalString("address"), host), "Invalid argument")

	minio.StartGateway(ctx, &Sia{host})
}

// Sia implements Gateway.
type Sia struct {
	host string // Sia daemon host address
}

// Name implements Gateway interface.
func (g *Sia) Name() string {
	return siaBackend
}

// NewGatewayLayer returns Sia gateway layer, implements ObjectLayer interface to
// talk to Sia backend.
func (g *Sia) NewGatewayLayer(creds auth.Credentials) (minio.ObjectLayer, error) {
	sia := &siaObjects{
		Address: g.host,
		// RootDir uses access key directly, provides partitioning for
		// concurrent users talking to same sia daemon.
		RootDir:  creds.AccessKey,
		TempDir:  os.Getenv("SIA_TEMP_DIR"),
		password: os.Getenv("SIA_API_PASSWORD"),
	}

	// If Address not provided on command line or ENV, default to:
	if sia.Address == "" {
		sia.Address = "127.0.0.1:9980"
	}

	// If local Sia temp directory not specified, default to:
	if sia.TempDir == "" {
		sia.TempDir = ".sia_temp"
	}

	var err error
	sia.TempDir, err = filepath.Abs(sia.TempDir)
	if err != nil {
		return nil, err
	}

	// Create the temp directory with proper permissions.
	// Ignore error when dir already exists.
	if err = os.MkdirAll(sia.TempDir, 0700); err != nil {
		return nil, err
	}

	colorBlue := color.New(color.FgBlue).SprintfFunc()
	colorBold := color.New(color.Bold).SprintFunc()

	formatStr := "%" + fmt.Sprintf("%ds", len(sia.Address)+7)
	logger.StartupMessage(colorBlue("\nSia Configuration:"))
	logger.StartupMessage(colorBlue("  API Address:") + colorBold(fmt.Sprintf(formatStr, sia.Address)))
	logger.StartupMessage(colorBlue("  Staging Directory:") + colorBold(fmt.Sprintf(" %s", sia.TempDir)))

	return sia, nil
}

// Production - sia gateway is not ready for production use.
func (g *Sia) Production() bool {
	return false
}

// non2xx returns true for non-success HTTP status codes.
func non2xx(code int) bool {
	return code < 200 || code > 299
}

// decodeError returns the api.Error from a API response. This method should
// only be called if the response's status code is non-2xx. The error returned
// may not be of type api.Error in the event of an error unmarshalling the
// JSON.
type siaError struct {
	// Message describes the error in English. Typically it is set to
	// `err.Error()`. This field is required.
	Message string `json:"message"`
}

func (s siaError) Error() string {
	return s.Message
}

func decodeError(resp *http.Response) error {
	// Error is a type that is encoded as JSON and returned in an API response in
	// the event of an error. Only the Message field is required. More fields may
	// be added to this struct in the future for better error reporting.
	var apiErr siaError
	if err := json.NewDecoder(resp.Body).Decode(&apiErr); err != nil {
		return err
	}
	return apiErr
}

// MethodNotSupported - returned if call returned error.
type MethodNotSupported struct {
	method string
}

func (s MethodNotSupported) Error() string {
	return fmt.Sprintf("API call not recognized: %s", s.method)
}

// apiGet wraps a GET request with a status code check, such that if the GET does
// not return 2xx, the error will be read and returned. The response body is
// not closed.
func apiGet(ctx context.Context, addr, call, apiPassword string) (*http.Response, error) {
	req, err := http.NewRequest("GET", "http://"+addr+call, nil)
	if err != nil {
		logger.LogIf(ctx, err)
		return nil, err
	}
	req.Header.Set("User-Agent", "Sia-Agent")
	if apiPassword != "" {
		req.SetBasicAuth("", apiPassword)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		logger.LogIf(ctx, err)
		return nil, err
	}
	if resp.StatusCode == http.StatusNotFound {
		minio.CloseResponse(resp.Body)
		logger.LogIf(ctx, MethodNotSupported{call})
		return nil, MethodNotSupported{call}
	}
	if non2xx(resp.StatusCode) {
		err := decodeError(resp)
		minio.CloseResponse(resp.Body)
		logger.LogIf(ctx, err)
		return nil, err
	}
	return resp, nil
}

// apiPost wraps a POST request with a status code check, such that if the POST
// does not return 2xx, the error will be read and returned. The response body
// is not closed.
func apiPost(ctx context.Context, addr, call, vals, apiPassword string) (*http.Response, error) {
	req, err := http.NewRequest("POST", "http://"+addr+call, strings.NewReader(vals))
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Sia-Agent")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if apiPassword != "" {
		req.SetBasicAuth("", apiPassword)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		logger.LogIf(ctx, err)
		return nil, err
	}

	if resp.StatusCode == http.StatusNotFound {
		minio.CloseResponse(resp.Body)
		return nil, MethodNotSupported{call}
	}

	if non2xx(resp.StatusCode) {
		err := decodeError(resp)
		minio.CloseResponse(resp.Body)
		return nil, err
	}
	return resp, nil
}

// post makes an API call and discards the response. An error is returned if
// the response status is not 2xx.
func post(ctx context.Context, addr, call, vals, apiPassword string) error {
	resp, err := apiPost(ctx, addr, call, vals, apiPassword)
	if err != nil {
		return err
	}
	minio.CloseResponse(resp.Body)
	return nil
}

// list makes a lists all the uploaded files, decodes the json response.
func list(ctx context.Context, addr string, apiPassword string, obj *renterFiles) error {
	resp, err := apiGet(ctx, addr, "/renter/files", apiPassword)
	if err != nil {
		return err
	}
	defer minio.CloseResponse(resp.Body)

	if resp.StatusCode == http.StatusNoContent {
		logger.LogIf(ctx, fmt.Errorf("Expecting a response, but API returned %s", resp.Status))
		return fmt.Errorf("Expecting a response, but API returned %s", resp.Status)
	}
	err = json.NewDecoder(resp.Body).Decode(obj)
	logger.LogIf(ctx, err)
	return err
}

// get makes an API call and discards the response. An error is returned if the
// responsee status is not 2xx.
func get(ctx context.Context, addr, call, apiPassword string) error {
	resp, err := apiGet(ctx, addr, call, apiPassword)
	if err != nil {
		return err
	}
	minio.CloseResponse(resp.Body)
	return nil
}

// Shutdown saves any gateway metadata to disk
// if necessary and reload upon next restart.
func (s *siaObjects) Shutdown(ctx context.Context) error {
	return nil
}

// StorageInfo is not relevant to Sia backend.
func (s *siaObjects) StorageInfo(ctx context.Context) (si minio.StorageInfo) {
	return si
}

// MakeBucket creates a new container on Sia backend.
func (s *siaObjects) MakeBucketWithLocation(ctx context.Context, bucket, location string) error {
	srcFile := path.Join(s.TempDir, minio.MustGetUUID())
	defer os.Remove(srcFile)

	writer, err := os.Create(srcFile)
	if err != nil {
		return err
	}
	if _, err = io.Copy(writer, bytes.NewReader([]byte(""))); err != nil {
		return err
	}

	sha256sum := sha256.Sum256([]byte(bucket))
	var siaObj = path.Join(s.RootDir, bucket, hex.EncodeToString(sha256sum[:]))
	return post(ctx, s.Address, "/renter/upload/"+siaObj, "source="+srcFile, s.password)
}

// GetBucketInfo gets bucket metadata.
func (s *siaObjects) GetBucketInfo(ctx context.Context, bucket string) (bi minio.BucketInfo, err error) {
	sha256sum := sha256.Sum256([]byte(bucket))
	var siaObj = path.Join(s.RootDir, bucket, hex.EncodeToString(sha256sum[:]))

	dstFile := path.Join(s.TempDir, minio.MustGetUUID())
	defer os.Remove(dstFile)

	if err := get(ctx, s.Address, "/renter/download/"+siaObj+"?destination="+url.QueryEscape(dstFile), s.password); err != nil {
		return bi, err
	}
	return minio.BucketInfo{Name: bucket}, nil
}

// ListBuckets will detect and return existing buckets on Sia.
func (s *siaObjects) ListBuckets(ctx context.Context) (buckets []minio.BucketInfo, err error) {
	sObjs, serr := s.listRenterFiles(ctx, "")
	if serr != nil {
		return buckets, serr
	}

	m := make(set.StringSet)

	prefix := s.RootDir + "/"
	for _, sObj := range sObjs {
		if strings.HasPrefix(sObj.SiaPath, prefix) {
			siaObj := strings.TrimPrefix(sObj.SiaPath, prefix)
			idx := strings.Index(siaObj, "/")
			if idx > 0 {
				m.Add(siaObj[0:idx])
			}
		}
	}

	for _, bktName := range m.ToSlice() {
		buckets = append(buckets, minio.BucketInfo{
			Name:    bktName,
			Created: time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC),
		})
	}

	return buckets, nil
}

// DeleteBucket deletes a bucket on Sia.
func (s *siaObjects) DeleteBucket(ctx context.Context, bucket string) error {
	sha256sum := sha256.Sum256([]byte(bucket))
	var siaObj = path.Join(s.RootDir, bucket, hex.EncodeToString(sha256sum[:]))

	return post(ctx, s.Address, "/renter/delete/"+siaObj, "", s.password)
}

func (s *siaObjects) ListObjects(ctx context.Context, bucket string, prefix string, marker string, delimiter string, maxKeys int) (loi minio.ListObjectsInfo, err error) {
	siaObjs, siaErr := s.listRenterFiles(ctx, bucket)
	if siaErr != nil {
		return loi, siaErr
	}

	loi.IsTruncated = false
	loi.NextMarker = ""

	root := s.RootDir + "/"

	sha256sum := sha256.Sum256([]byte(bucket))
	// FIXME(harsha) - No paginated output supported for Sia backend right now, only prefix
	// based filtering. Once list renter files API supports paginated output we can support
	// paginated results here as well - until then Listing is an expensive operation.
	for _, sObj := range siaObjs {
		name := strings.TrimPrefix(sObj.SiaPath, path.Join(root, bucket)+"/")
		// Skip the file created specially when bucket was created.
		if name == hex.EncodeToString(sha256sum[:]) {
			continue
		}
		if strings.HasPrefix(name, prefix) {
			loi.Objects = append(loi.Objects, minio.ObjectInfo{
				Bucket: bucket,
				Name:   name,
				Size:   int64(sObj.Filesize),
				IsDir:  false,
			})
		}
	}
	return loi, nil
}

// GetObjectNInfo - returns object info and locked object ReadCloser
func (s *siaObjects) GetObjectNInfo(ctx context.Context, bucket, object string, rs *minio.HTTPRangeSpec, h http.Header, lockType minio.LockType, opts minio.ObjectOptions) (gr *minio.GetObjectReader, err error) {
	var objInfo minio.ObjectInfo
	objInfo, err = s.GetObjectInfo(ctx, bucket, object, opts)
	if err != nil {
		return nil, err
	}

	var startOffset, length int64
	startOffset, length, err = rs.GetOffsetLength(objInfo.Size)
	if err != nil {
		return nil, err
	}

	pr, pw := io.Pipe()
	go func() {
		err := s.GetObject(ctx, bucket, object, startOffset, length, pw, objInfo.ETag, opts)
		pw.CloseWithError(err)
	}()
	// Setup cleanup function to cause the above go-routine to
	// exit in case of partial read
	pipeCloser := func() { pr.Close() }
	return minio.NewGetObjectReaderFromReader(pr, objInfo, pipeCloser), nil
}

func (s *siaObjects) GetObject(ctx context.Context, bucket string, object string, startOffset int64, length int64, writer io.Writer, etag string, opts minio.ObjectOptions) error {
	dstFile := path.Join(s.TempDir, minio.MustGetUUID())
	defer os.Remove(dstFile)

	var siaObj = path.Join(s.RootDir, bucket, object)
	if err := get(ctx, s.Address, "/renter/download/"+siaObj+"?destination="+url.QueryEscape(dstFile), s.password); err != nil {
		return err
	}

	reader, err := os.Open(dstFile)
	if err != nil {
		return err
	}
	defer reader.Close()
	st, err := reader.Stat()
	if err != nil {
		return err
	}
	size := st.Size()
	if _, err = reader.Seek(startOffset, os.SEEK_SET); err != nil {
		return err
	}

	// For negative length we read everything.
	if length < 0 {
		length = size - startOffset
	}

	bufSize := int64(1 * humanize.MiByte)
	if bufSize > length {
		bufSize = length
	}

	// Reply back invalid range if the input offset and length fall out of range.
	if startOffset > size || startOffset+length > size {
		logger.LogIf(ctx, minio.InvalidRange{
			OffsetBegin:  startOffset,
			OffsetEnd:    length,
			ResourceSize: size,
		})
		return minio.InvalidRange{
			OffsetBegin:  startOffset,
			OffsetEnd:    length,
			ResourceSize: size,
		}
	}

	// Allocate a staging buffer.
	buf := make([]byte, int(bufSize))

	_, err = io.CopyBuffer(writer, io.LimitReader(reader, length), buf)

	return err
}

// findSiaObject retrieves the siaObjectInfo for the Sia object with the given
// Sia path name.
func (s *siaObjects) findSiaObject(ctx context.Context, bucket, object string) (siaObjectInfo, error) {
	siaPath := path.Join(s.RootDir, bucket, object)

	sObjs, err := s.listRenterFiles(ctx, "")
	if err != nil {
		return siaObjectInfo{}, err
	}

	for _, sObj := range sObjs {
		if sObj.SiaPath == siaPath {
			return sObj, nil
		}
	}
	logger.LogIf(ctx, minio.ObjectNotFound{
		Bucket: bucket,
		Object: object,
	})
	return siaObjectInfo{}, minio.ObjectNotFound{
		Bucket: bucket,
		Object: object,
	}
}

// GetObjectInfo reads object info and replies back ObjectInfo
func (s *siaObjects) GetObjectInfo(ctx context.Context, bucket string, object string, opts minio.ObjectOptions) (minio.ObjectInfo, error) {
	so, err := s.findSiaObject(ctx, bucket, object)
	if err != nil {
		return minio.ObjectInfo{}, err
	}

	// Metadata about sia objects is just quite minimal. Sia only provides file size.
	return minio.ObjectInfo{
		Bucket:  bucket,
		Name:    object,
		ModTime: time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC),
		Size:    int64(so.Filesize),
		IsDir:   false,
	}, nil
}

// PutObject creates a new object with the incoming data,
func (s *siaObjects) PutObject(ctx context.Context, bucket string, object string, data *hash.Reader, metadata map[string]string, opts minio.ObjectOptions) (objInfo minio.ObjectInfo, err error) {
	srcFile := path.Join(s.TempDir, minio.MustGetUUID())
	writer, err := os.Create(srcFile)
	if err != nil {
		return objInfo, err
	}

	wsize, err := io.CopyN(writer, data, data.Size())
	if err != nil {
		os.Remove(srcFile)
		return objInfo, err
	}

	if err = post(ctx, s.Address, "/renter/upload/"+path.Join(s.RootDir, bucket, object), "source="+srcFile, s.password); err != nil {
		os.Remove(srcFile)
		return objInfo, err
	}
	defer s.deleteTempFileWhenUploadCompletes(ctx, srcFile, bucket, object)

	return minio.ObjectInfo{
		Name:    object,
		Bucket:  bucket,
		ModTime: time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC),
		Size:    wsize,
		ETag:    minio.GenETag(),
	}, nil
}

// DeleteObject deletes a blob in bucket
func (s *siaObjects) DeleteObject(ctx context.Context, bucket string, object string) error {
	// Tell Sia daemon to delete the object
	var siaObj = path.Join(s.RootDir, bucket, object)
	return post(ctx, s.Address, "/renter/delete/"+siaObj, "", s.password)
}

// siaObjectInfo represents object info stored on Sia
type siaObjectInfo struct {
	SiaPath        string  `json:"siapath"`
	LocalPath      string  `json:"localpath"`
	Filesize       uint64  `json:"filesize"`
	Available      bool    `json:"available"`
	Renewing       bool    `json:"renewing"`
	Redundancy     float64 `json:"redundancy"`
	UploadProgress float64 `json:"uploadprogress"`
}

type renterFiles struct {
	Files []siaObjectInfo `json:"files"`
}

// listRenterFiles will return a list of existing objects in the bucket provided
func (s *siaObjects) listRenterFiles(ctx context.Context, bucket string) (siaObjs []siaObjectInfo, err error) {
	// Get list of all renter files
	var rf renterFiles
	if err = list(ctx, s.Address, s.password, &rf); err != nil {
		return siaObjs, err
	}

	var prefix string
	root := s.RootDir + "/"
	if bucket == "" {
		prefix = root
	} else {
		prefix = root + bucket + "/"
	}

	for _, f := range rf.Files {
		if strings.HasPrefix(f.SiaPath, prefix) {
			siaObjs = append(siaObjs, f)
		}
	}

	return siaObjs, nil
}

// deleteTempFileWhenUploadCompletes checks the status of a Sia file upload
// until it reaches 100% upload progress, then deletes the local temp copy from
// the filesystem.
func (s *siaObjects) deleteTempFileWhenUploadCompletes(ctx context.Context, tempFile string, bucket, object string) {
	var soi siaObjectInfo
	// Wait until 100% upload instead of 1x redundancy because if we delete
	// after 1x redundancy, the user has to pay the cost of other hosts
	// redistributing the file.
	for soi.UploadProgress < 100.0 {
		var err error
		soi, err = s.findSiaObject(ctx, bucket, object)
		if err != nil {
			break
		}

		// Sleep between each check so that we're not hammering
		// the Sia daemon with requests.
		time.Sleep(15 * time.Second)
	}

	os.Remove(tempFile)
}

// IsCompressionSupported returns whether compression is applicable for this layer.
func (s *siaObjects) IsCompressionSupported() bool {
	return false
}
