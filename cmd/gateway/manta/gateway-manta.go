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

package manta

import (
	"context"
	"encoding/pem"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"strings"

	triton "github.com/joyent/triton-go"
	"github.com/joyent/triton-go/authentication"
	terrors "github.com/joyent/triton-go/errors"
	"github.com/joyent/triton-go/storage"
	"github.com/minio/cli"
	minio "github.com/minio/minio/cmd"
	"github.com/minio/minio/cmd/logger"
	"github.com/minio/minio/pkg/auth"
	"github.com/minio/minio/pkg/hash"
)

// stor is a namespace within manta where you store any documents that are deemed as private
// and require access credentials to read them. Within the stor namespace, you can create any
// number of directories and objects.
const (
	mantaBackend     = "manta"
	defaultMantaRoot = "/stor"
	defaultMantaURL  = "https://us-east.manta.joyent.com"
)

var mantaRoot = defaultMantaRoot

func init() {
	const mantaGatewayTemplate = `NAME:
  {{.HelpName}} - {{.Usage}}
USAGE:
  {{.HelpName}} {{if .VisibleFlags}}[FLAGS]{{end}} [ENDPOINT]
{{if .VisibleFlags}}
FLAGS:
  {{range .VisibleFlags}}{{.}}
  {{end}}{{end}}
ENDPOINT:
  Manta server endpoint. Default ENDPOINT is https://us-east.manta.joyent.com

ENVIRONMENT VARIABLES:
  ACCESS:
     MINIO_ACCESS_KEY: The Manta account name.
     MINIO_SECRET_KEY: A KeyID associated with the Manta account.
     MANTA_KEY_MATERIAL: The path to the SSH Key associated with the Manta account if the MINIO_SECRET_KEY is not in SSH Agent.
	 MANTA_SUBUSER: The username of a user who has limited access to your account.

  BROWSER:
     MINIO_BROWSER: To disable web browser access, set this value to "off".

  DOMAIN:
     MINIO_DOMAIN: To enable virtual-host-style requests, set this value to Minio host domain name.

  CACHE:
     MINIO_CACHE_DRIVES: List of mounted drives or directories delimited by ";".
     MINIO_CACHE_EXCLUDE: List of cache exclusion patterns delimited by ";".
     MINIO_CACHE_EXPIRY: Cache expiry duration in days.
     MINIO_CACHE_MAXUSE: Maximum permitted usage of the cache in percentage (0-100).

EXAMPLES:
  1. Start minio gateway server for Manta Object Storage backend.
     $ export MINIO_ACCESS_KEY=manta_account_name
     $ export MINIO_SECRET_KEY=manta_key_id
     $ {{.HelpName}}

  2. Start minio gateway server for Manta Object Storage backend on custom endpoint.
     $ export MINIO_ACCESS_KEY=manta_account_name
     $ export MINIO_SECRET_KEY=manta_key_id
     $ {{.HelpName}} https://us-west.manta.joyent.com

  3. Start minio gateway server for Manta Object Storage backend without using SSH Agent.
     $ export MINIO_ACCESS_KEY=manta_account_name
     $ export MINIO_SECRET_KEY=manta_key_id
     $ export MANTA_KEY_MATERIAL=~/.ssh/custom_rsa
     $ {{.HelpName}}

  4. Start minio gateway server for Manta Object Storage backend with edge caching enabled.
     $ export MINIO_ACCESS_KEY=manta_account_name
     $ export MINIO_SECRET_KEY=manta_key_id
     $ export MINIO_CACHE_DRIVES="/mnt/drive1;/mnt/drive2;/mnt/drive3;/mnt/drive4"
     $ export MINIO_CACHE_EXCLUDE="bucket1/*;*.png"
     $ export MINIO_CACHE_EXPIRY=40
     $ export MINIO_CACHE_MAXUSE=80
     $ {{.HelpName}}
`

	minio.RegisterGatewayCommand(cli.Command{
		Name:               mantaBackend,
		Usage:              "Manta Object Storage.",
		Action:             mantaGatewayMain,
		CustomHelpTemplate: mantaGatewayTemplate,
		HideHelpCommand:    true,
	})
}

func mantaGatewayMain(ctx *cli.Context) {
	args := ctx.Args()
	if !ctx.Args().Present() {
		args = cli.Args{"https://us-east.manta.joyent.com"}
	}

	// Validate gateway arguments.
	logger.FatalIf(minio.ValidateGatewayArguments(ctx.GlobalString("address"), args.First()), "Invalid argument")

	// Start the gateway..
	minio.StartGateway(ctx, &Manta{args.First()})
}

// Manta implements Gateway.
type Manta struct {
	host string
}

// Name implements Gateway interface.
func (g *Manta) Name() string {
	return mantaBackend
}

// NewGatewayLayer returns manta gateway layer, implements ObjectLayer interface to
// talk to manta remote backend.
func (g *Manta) NewGatewayLayer(creds auth.Credentials) (minio.ObjectLayer, error) {
	var err error
	var secure bool
	var signer authentication.Signer
	var endpoint = defaultMantaURL
	ctx := context.Background()

	if g.host != "" {
		endpoint, secure, err = minio.ParseGatewayEndpoint(g.host)
		if err != nil {
			return nil, err
		}
		if secure {
			endpoint = "https://" + endpoint
		} else {
			endpoint = "http://" + endpoint
		}
	}
	if overrideRoot, ok := os.LookupEnv("MANTA_ROOT"); ok {
		mantaRoot = overrideRoot
	}

	keyMaterial := os.Getenv("MANTA_KEY_MATERIAL")

	if keyMaterial == "" {
		input := authentication.SSHAgentSignerInput{
			KeyID:       creds.SecretKey,
			AccountName: creds.AccessKey,
		}
		if userName, ok := os.LookupEnv("MANTA_SUBUSER"); ok {
			input.Username = userName
		}
		signer, err = authentication.NewSSHAgentSigner(input)
		if err != nil {
			logger.LogIf(ctx, err)
			return nil, err
		}
	} else {
		var keyBytes []byte
		if _, err = os.Stat(keyMaterial); err == nil {
			keyBytes, err = ioutil.ReadFile(keyMaterial)
			if err != nil {
				return nil, fmt.Errorf("Error reading key material from %s: %s",
					keyMaterial, err)
			}
			block, _ := pem.Decode(keyBytes)
			if block == nil {
				return nil, fmt.Errorf(
					"Failed to read key material '%s': no key found", keyMaterial)
			}

			if block.Headers["Proc-Type"] == "4,ENCRYPTED" {
				return nil, fmt.Errorf(
					"Failed to read key '%s': password protected keys are\n"+
						"not currently supported. Please decrypt the key prior to use.", keyMaterial)
			}

		} else {
			keyBytes = []byte(keyMaterial)
		}

		input := authentication.PrivateKeySignerInput{
			KeyID:              creds.SecretKey,
			PrivateKeyMaterial: keyBytes,
			AccountName:        creds.AccessKey,
		}
		if userName, ok := os.LookupEnv("MANTA_SUBUSER"); ok {
			input.Username = userName
		}

		signer, err = authentication.NewPrivateKeySigner(input)
		if err != nil {
			logger.LogIf(ctx, err)
			return nil, err
		}
	}

	tc, err := storage.NewClient(&triton.ClientConfig{
		MantaURL:    endpoint,
		AccountName: creds.AccessKey,
		Signers:     []authentication.Signer{signer},
	})
	if err != nil {
		return nil, err
	}

	tc.Client.HTTPClient = &http.Client{
		Transport: minio.NewCustomHTTPTransport(),
	}

	return &tritonObjects{
		client: tc,
	}, nil
}

// Production - Manta is production ready.
func (g *Manta) Production() bool {
	return true
}

// tritonObjects - Implements Object layer for Triton Manta storage
type tritonObjects struct {
	minio.GatewayUnsupported
	client *storage.StorageClient
}

// Shutdown - save any gateway metadata to disk
// if necessary and reload upon next restart.
func (t *tritonObjects) Shutdown(ctx context.Context) error {
	return nil
}

// StorageInfo - Not relevant to Triton backend.
func (t *tritonObjects) StorageInfo(ctx context.Context) (si minio.StorageInfo) {
	return si
}

//
// ~~~ Buckets ~~~
//

// MakeBucketWithLocation - Create a new directory within manta.
//
// https://apidocs.joyent.com/manta/api.html#PutDirectory
func (t *tritonObjects) MakeBucketWithLocation(ctx context.Context, bucket, location string) error {
	err := t.client.Dir().Put(ctx, &storage.PutDirectoryInput{
		DirectoryName: path.Join(mantaRoot, bucket),
	})
	if err != nil {
		return err
	}
	return nil
}

// GetBucketInfo - Get directory metadata..
//
// https://apidocs.joyent.com/manta/api.html#GetObject
func (t *tritonObjects) GetBucketInfo(ctx context.Context, bucket string) (bi minio.BucketInfo, e error) {
	var info minio.BucketInfo
	resp, err := t.client.Objects().Get(ctx, &storage.GetObjectInput{
		ObjectPath: path.Join(mantaRoot, bucket),
	})
	if err != nil {
		return info, err
	}

	return minio.BucketInfo{
		Name:    bucket,
		Created: resp.LastModified,
	}, nil
}

// ListBuckets - Lists all Manta directories, uses Manta equivalent
// ListDirectories.
//
// https://apidocs.joyent.com/manta/api.html#ListDirectory
func (t *tritonObjects) ListBuckets(ctx context.Context) (buckets []minio.BucketInfo, err error) {
	dirs, err := t.client.Dir().List(ctx, &storage.ListDirectoryInput{
		DirectoryName: path.Join(mantaRoot),
	})
	if err != nil {
		return nil, err
	}

	for _, dir := range dirs.Entries {
		if dir.Type == "directory" {
			buckets = append(buckets, minio.BucketInfo{
				Name:    dir.Name,
				Created: dir.ModifiedTime,
			})
		}
	}

	return buckets, nil
}

// DeleteBucket - Delete a directory in Manta, uses Manta equivalent
// DeleteDirectory.
//
// https://apidocs.joyent.com/manta/api.html#DeleteDirectory
func (t *tritonObjects) DeleteBucket(ctx context.Context, bucket string) error {
	return t.client.Dir().Delete(ctx, &storage.DeleteDirectoryInput{
		DirectoryName: path.Join(mantaRoot, bucket),
	})
}

//
// ~~~ Objects ~~~
//

// ListObjects - Lists all objects in Manta with a container filtered by prefix
// and marker, uses Manta equivalent ListDirectory.
//
// https://apidocs.joyent.com/manta/api.html#ListDirectory
func (t *tritonObjects) ListObjects(ctx context.Context, bucket, prefix, marker, delimiter string, maxKeys int) (result minio.ListObjectsInfo, err error) {
	var (
		dirName string
		objs    *storage.ListDirectoryOutput
		input   *storage.ListDirectoryInput

		pathBase = path.Base(prefix)
	)

	// Make sure to only request a Dir.List for the parent "directory" for a
	// given prefix first. We don't know if our prefix is referencing a
	// directory or file name and can't send file names into Dir.List because
	// that'll cause Manta to return file content in the response body. Dir.List
	// expects to parse out directory entries in JSON. So, try the first
	// directory name of the prefix path provided.
	if pathDir := path.Dir(prefix); pathDir == "." {
		dirName = path.Join(mantaRoot, bucket)
	} else {
		dirName = path.Join(mantaRoot, bucket, pathDir)
	}

	if marker != "" {
		// Manta uses the marker as the key to start at rather than start after
		// A space is appended to the marker so that the corresponding object is not
		// included in the results
		marker += " "
	}

	input = &storage.ListDirectoryInput{
		DirectoryName: dirName,
		Limit:         uint64(maxKeys),
		Marker:        marker,
	}
	objs, err = t.client.Dir().List(ctx, input)
	if err != nil {
		if terrors.IsResourceNotFoundError(err) {
			return result, nil
		}
		logger.LogIf(ctx, err)
		return result, err
	}

	for _, obj := range objs.Entries {
		// If the base name of our prefix was found to be of type "directory"
		// than we need to pull the directory entries for that instead.
		if obj.Name == pathBase && obj.Type == "directory" {
			input.DirectoryName = path.Join(mantaRoot, bucket, prefix)
			objs, err = t.client.Dir().List(ctx, input)
			if err != nil {
				logger.LogIf(ctx, err)
				return result, err
			}
			break
		}
	}

	isTruncated := true // Always send a second request.
	if marker == "" && len(objs.Entries) < maxKeys {
		isTruncated = false
	} else if marker != "" && len(objs.Entries) < maxKeys {
		isTruncated = false
	}

	for _, obj := range objs.Entries {
		if obj.Type == "directory" {
			result.Prefixes = append(result.Prefixes, obj.Name+delimiter)
		} else {
			result.Objects = append(result.Objects, minio.ObjectInfo{
				Name:    obj.Name,
				Size:    int64(obj.Size),
				ModTime: obj.ModifiedTime,
				ETag:    obj.ETag,
			})
		}
	}

	result.IsTruncated = isTruncated
	if isTruncated {
		result.NextMarker = result.Objects[len(result.Objects)-1].Name
	}
	return result, nil
}

//
// ~~~ Objects ~~~
//

// ListObjectsV2 - Lists all objects in Manta with a container filtered by prefix
// and continuationToken, uses Manta equivalent ListDirectory.
//
// https://apidocs.joyent.com/manta/api.html#ListDirectory
func (t *tritonObjects) ListObjectsV2(ctx context.Context, bucket, prefix, continuationToken, delimiter string, maxKeys int, fetchOwner bool, startAfter string) (result minio.ListObjectsV2Info, err error) {
	var (
		dirName string
		objs    *storage.ListDirectoryOutput
		input   *storage.ListDirectoryInput

		pathBase = path.Base(prefix)
	)

	marker := continuationToken
	if marker == "" {
		marker = startAfter
	}

	if marker != "" {
		// Manta uses the marker as the key to start at rather than start after.
		// A space is appended to the marker so that the corresponding object is not
		// included in the results
		marker += " "
	}

	if pathDir := path.Dir(prefix); pathDir == "." {
		dirName = path.Join(mantaRoot, bucket)
	} else {
		dirName = path.Join(mantaRoot, bucket, pathDir)
	}

	input = &storage.ListDirectoryInput{
		DirectoryName: dirName,
		Limit:         uint64(maxKeys),
		Marker:        marker,
	}
	objs, err = t.client.Dir().List(ctx, input)
	if err != nil {
		if terrors.IsResourceNotFoundError(err) {
			return result, nil
		}
		logger.LogIf(ctx, err)
		return result, err
	}

	for _, obj := range objs.Entries {
		if obj.Name == pathBase && obj.Type == "directory" {
			input.DirectoryName = path.Join(mantaRoot, bucket, prefix)
			objs, err = t.client.Dir().List(ctx, input)
			if err != nil {
				logger.LogIf(ctx, err)
				return result, err
			}
			break
		}
	}

	isTruncated := true // Always send a second request.
	if continuationToken == "" && len(objs.Entries) < maxKeys {
		isTruncated = false
	} else if continuationToken != "" && len(objs.Entries) < maxKeys {
		isTruncated = false
	}

	for _, obj := range objs.Entries {
		if obj.Type == "directory" {
			result.Prefixes = append(result.Prefixes, obj.Name+delimiter)
		} else {
			result.Objects = append(result.Objects, minio.ObjectInfo{
				Name:    obj.Name,
				Size:    int64(obj.Size),
				ModTime: obj.ModifiedTime,
				ETag:    obj.ETag,
			})
		}
	}

	result.IsTruncated = isTruncated
	if isTruncated {
		result.NextContinuationToken = result.Objects[len(result.Objects)-1].Name
	}
	return result, nil
}

// GetObjectNInfo - returns object info and locked object ReadCloser
func (t *tritonObjects) GetObjectNInfo(ctx context.Context, bucket, object string, rs *minio.HTTPRangeSpec, h http.Header, lockType minio.LockType, opts minio.ObjectOptions) (gr *minio.GetObjectReader, err error) {
	var objInfo minio.ObjectInfo
	objInfo, err = t.GetObjectInfo(ctx, bucket, object, opts)
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
		err := t.GetObject(ctx, bucket, object, startOffset, length, pw, objInfo.ETag, opts)
		pw.CloseWithError(err)
	}()
	// Setup cleanup function to cause the above go-routine to
	// exit in case of partial read
	pipeCloser := func() { pr.Close() }
	return minio.NewGetObjectReaderFromReader(pr, objInfo, pipeCloser), nil
}

// GetObject - Reads an object from Manta. Supports additional parameters like
// offset and length which are synonymous with HTTP Range requests.
//
// startOffset indicates the starting read location of the object.  length
// indicates the total length of the object.
//
// https://apidocs.joyent.com/manta/api.html#GetObject
func (t *tritonObjects) GetObject(ctx context.Context, bucket, object string, startOffset int64, length int64, writer io.Writer, etag string, opts minio.ObjectOptions) error {
	// Start offset cannot be negative.
	if startOffset < 0 {
		logger.LogIf(ctx, fmt.Errorf("Unexpected error"))
		return fmt.Errorf("Unexpected error")
	}

	output, err := t.client.Objects().Get(ctx, &storage.GetObjectInput{
		ObjectPath: path.Join(mantaRoot, bucket, object),
	})
	if err != nil {
		return err
	}
	defer output.ObjectReader.Close()

	// Read until startOffset and discard, Manta object storage doesn't support range GET requests yet.
	if _, err = io.CopyN(ioutil.Discard, output.ObjectReader, startOffset); err != nil {
		return err
	}

	if length > 0 {
		_, err = io.Copy(writer, io.LimitReader(output.ObjectReader, length))
	} else {
		_, err = io.Copy(writer, output.ObjectReader)
	}
	return err
}

// GetObjectInfo - reads blob metadata properties and replies back minio.ObjectInfo,
// uses Triton equivalent GetBlobProperties.
//
// https://apidocs.joyent.com/manta/api.html#GetObject
func (t *tritonObjects) GetObjectInfo(ctx context.Context, bucket, object string, opts minio.ObjectOptions) (objInfo minio.ObjectInfo, err error) {
	info, err := t.client.Objects().GetInfo(ctx, &storage.GetInfoInput{
		ObjectPath: path.Join(mantaRoot, bucket, object),
	})
	if err != nil {
		if terrors.IsStatusNotFoundCode(err) {
			return objInfo, minio.ObjectNotFound{
				Bucket: bucket,
				Object: object,
			}
		}

		return objInfo, err
	}

	return minio.ObjectInfo{
		Bucket:      bucket,
		ContentType: info.ContentType,
		Size:        int64(info.ContentLength),
		ETag:        info.ETag,
		ModTime:     info.LastModified,
		UserDefined: info.Metadata,
		IsDir:       strings.HasSuffix(info.ContentType, "type=directory"),
	}, nil
}

type dummySeeker struct {
	io.Reader
}

func (d dummySeeker) Seek(offset int64, whence int) (int64, error) {
	return 0, nil
}

// PutObject - Create a new blob with the incoming data, uses Triton equivalent
// CreateBlockBlobFromReader.
//
// https://apidocs.joyent.com/manta/api.html#PutObject
func (t *tritonObjects) PutObject(ctx context.Context, bucket, object string, data *hash.Reader, metadata map[string]string, opts minio.ObjectOptions) (objInfo minio.ObjectInfo, err error) {
	if err = t.client.Objects().Put(ctx, &storage.PutObjectInput{
		ContentLength: uint64(data.Size()),
		ObjectPath:    path.Join(mantaRoot, bucket, object),
		ContentType:   metadata["content-type"],
		// TODO: Change to `string(data.md5sum)` if/when that becomes an exported field
		ContentMD5:   metadata["content-md5"],
		ObjectReader: dummySeeker{data},
		ForceInsert:  true,
	}); err != nil {
		logger.LogIf(ctx, err)
		return objInfo, err
	}
	if err = data.Verify(); err != nil {
		t.DeleteObject(ctx, bucket, object)
		logger.LogIf(ctx, err)
		return objInfo, err
	}

	return t.GetObjectInfo(ctx, bucket, object, opts)
}

// CopyObject - Copies a blob from source container to destination container.
// Uses Manta Snaplinks API.
//
// https://apidocs.joyent.com/manta/api.html#PutSnapLink
func (t *tritonObjects) CopyObject(ctx context.Context, srcBucket, srcObject, destBucket, destObject string, srcInfo minio.ObjectInfo, srcOpts, dstOpts minio.ObjectOptions) (objInfo minio.ObjectInfo, err error) {
	if err = t.client.SnapLinks().Put(ctx, &storage.PutSnapLinkInput{
		SourcePath: path.Join(mantaRoot, srcBucket, srcObject),
		LinkPath:   path.Join(mantaRoot, destBucket, destObject),
	}); err != nil {
		logger.LogIf(ctx, err)
		return objInfo, err
	}

	return t.GetObjectInfo(ctx, destBucket, destObject, dstOpts)
}

// DeleteObject - Delete a blob in Manta, uses Triton equivalent DeleteBlob API.
//
// https://apidocs.joyent.com/manta/api.html#DeleteObject
func (t *tritonObjects) DeleteObject(ctx context.Context, bucket, object string) error {
	if err := t.client.Objects().Delete(ctx, &storage.DeleteObjectInput{
		ObjectPath: path.Join(mantaRoot, bucket, object),
	}); err != nil {
		logger.LogIf(ctx, err)
		return err
	}

	return nil
}

// IsCompressionSupported returns whether compression is applicable for this layer.
func (t *tritonObjects) IsCompressionSupported() bool {
	return false
}
