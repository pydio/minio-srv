/*
 * Minio Cloud Storage, (C) 2017 Minio, Inc.
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
	"io"
	"path"

	"encoding/hex"

	"errors"
	minio "github.com/minio/minio-go"
	"github.com/pydio/services/common"
	"github.com/pydio/services/common/proto/tree"
	"golang.org/x/net/context"
	"strings"
	"time"

	"github.com/micro/go-plugins/client/grpc"
	"sync"
)

// s3Objects implements gateway for Minio and S3 compatible object storage servers.
type pydioObjects struct {
	Clients     map[string]*minio.Core
	anonClients map[string]*minio.Core
	TreeClient  tree.NodeProviderClient
	dsBuckets   map[string]string

	configMutex *sync.Mutex
}

// newS3Gateway returns s3 gatewaylayer
func newPydioGateway() (GatewayLayer, error) {

	clients := make(map[string]*minio.Core)
	anonClients := make(map[string]*minio.Core)
	dsBuckets := make(map[string]string)

	api := &pydioObjects{
		Clients:     clients,
		anonClients: anonClients,
		dsBuckets:   dsBuckets,
		TreeClient:  tree.NewNodeProviderClient(common.SERVICE_TREE, grpc.NewClient()),
		configMutex: &sync.Mutex{},
	}

	api.listDatasources()
	go api.watchRegistry()

	return api, nil
}

// Shutdown saves any gateway metadata to disk
// if necessary and reload upon next restart.
func (l *pydioObjects) Shutdown() error {
	// TODO
	return nil
}

// StorageInfo is not relevant to S3 backend.
func (l *pydioObjects) StorageInfo() (si StorageInfo) {
	return si
}

// GetBucketInfo gets bucket metadata..
func (l *pydioObjects) GetBucketInfo(bucket string) (bi BucketInfo, e error) {

	if bucket != "pydio" {
		return bi, traceError(BucketNotFound{Bucket: bucket})
	}
	return BucketInfo{
		Name:    bucket,
		Created: time.Now(),
	}, nil

	/*
		if _, ok := l.Clients[bucket]; ok{

			return BucketInfo{
				Name:    bucket,
				Created: time.Now(),
			}, nil

		}

		return bi, traceError(BucketNotFound{Bucket: bucket})
	*/
}

// ListBuckets lists all S3 buckets
func (l *pydioObjects) ListBuckets() ([]BucketInfo, error) {

	b := make([]BucketInfo, 1)
	b[0] = BucketInfo{
		Name:    "pydio",
		Created: time.Now(),
	}
	return b, nil

	/*
		i := 0
		for bucketName, _ := range l.Clients {
			b[i] = BucketInfo{
				Name:    bucketName,
				Created: time.Now(),
			}
			i++
		}
		return b, nil
	*/

}

// ListObjects lists all blobs in S3 bucket filtered by prefix
func (l *pydioObjects) ListObjects(bucket string, prefix string, marker string, delimiter string, maxKeys int) (loi ListObjectsInfo, e error) {

	objects, prefixes, err := l.ListPydioObjects(bucket, prefix, delimiter, maxKeys)
	if err != nil {
		return loi, s3ToObjectError(traceError(err), bucket)
	}

	log.Printf("\n[ListObjects] Returning %d objects and %d prefixes (V1) for prefix ", len(objects), len(prefixes), prefix)

	return ListObjectsInfo{
		IsTruncated: false,
		NextMarker:  "",
		Prefixes:    prefixes,
		Objects:     objects,
	}, nil

}

// ListObjectsV2 lists all blobs in S3 bucket filtered by prefix
func (l *pydioObjects) ListObjectsV2(bucket, prefix, continuationToken string, fetchOwner bool, delimiter string, maxKeys int) (loi ListObjectsV2Info, e error) {

	objects, prefixes, err := l.ListPydioObjects(bucket, prefix, delimiter, maxKeys)
	if err != nil {
		return loi, s3ToObjectError(traceError(err), bucket)
	}

	log.Printf("\n[ListObjectsV2] Returning %d objects and %d prefixes (V2) for prefix ", len(objects), len(prefixes), prefix)

	return ListObjectsV2Info{
		IsTruncated: false,
		Prefixes:    prefixes,
		Objects:     objects,

		ContinuationToken:     "",
		NextContinuationToken: "",
	}, nil

}

// GetObjectInfo reads object info and replies back ObjectInfo
func (l *pydioObjects) GetObjectInfo(bucket string, object string) (objInfo ObjectInfo, err error) {

	log.Println("GetObjectInfo : " + object)

	dataSourceName, newPrefix := l.prefixToDataSourceName(object)
	log.Println("GetObjectInfo :", dataSourceName, newPrefix)
	if newPrefix == "" {
		// This is a datasource object info
		return ObjectInfo{
			Bucket:  bucket,
			Name:    object,
			ModTime: time.Now(),
			Size:    0,
		}, nil

	}
	if dataSourceName == common.PYDIO_THUMBSTORE_NAMESPACE {
		// Use the thumb S3 client
		if thumbClient, ok := l.findMinioClientFor(bucket, object); ok {
			buck, obj := l.translateBucketAndPrefix(bucket, object)
			return l.getS3ObjectInfo(thumbClient, buck, obj)
		} else {
			return ObjectInfo{}, errors.New("Cannot find client for ThumbStore")
		}
	}

	treePath := strings.TrimLeft(object, "/")
	readNodeResponse, err := l.TreeClient.ReadNode(context.Background(), &tree.ReadNodeRequest{
		Node: &tree.Node{
			Path: treePath,
		},
	})

	if err != nil || readNodeResponse.Node == nil {

		archiveInfo, noArch := l.HeadFakeArchiveObject(bucket, object, dataSourceName)
		if noArch == nil {
			return archiveInfo, nil
		}

		return ObjectInfo{}, s3ToObjectError(traceError(&ObjectNotFound{}))
	}
	//log.Printf("Returning a node %v", readNodeResponse.Node)

	return fromPydioNodeObjectInfo(bucket, dataSourceName, readNodeResponse.Node), nil

}

// GetObjectInfo reads object info and replies back ObjectInfo
func (l *pydioObjects) getS3ObjectInfo(client *minio.Core, bucket string, object string) (objInfo ObjectInfo, err error) {
	r := minio.NewHeadReqHeaders()
	oi, err := client.StatObject(bucket, object, r)
	if err != nil {
		return ObjectInfo{}, s3ToObjectError(traceError(err), bucket, object)
	}

	return fromMinioClientObjectInfo(bucket, oi), nil
}

// GetObject reads an object from S3. Supports additional
// parameters like offset and length which are synonymous with
// HTTP Range requests.
//
// startOffset indicates the starting read location of the object.
// length indicates the total length of the object.
func (l *pydioObjects) GetObject(bucket string, key string, startOffset int64, length int64, writer io.Writer) error {

	log.Println("[GetObject]", bucket, key, startOffset, length)
	r := minio.NewGetReqHeaders()

	if length < 0 && length != -1 {
		return s3ToObjectError(traceError(errInvalidArgument), bucket, key)
	}

	if startOffset >= 0 && length >= 0 {
		if err := r.SetRange(startOffset, startOffset+length-1); err != nil {
			return s3ToObjectError(traceError(err), bucket, key)
		}
	}
	if client, ok := l.findMinioClientFor(bucket, key); ok {

		newBucket, newKey := l.translateBucketAndPrefix(bucket, key)
		objectReader, _, err := client.GetObject(newBucket, newKey, r)
		if err != nil {
			archive, err := l.GenerateArchiveFromKey(writer, bucket, key)
			if archive {
				return err
			} else {
				return s3ToObjectError(traceError(err), bucket, key)
			}
		}

		defer objectReader.Close()

		if _, err := io.Copy(writer, objectReader); err != nil {
			return s3ToObjectError(traceError(err), bucket, key)
		}

		return nil
	}

	return s3ToObjectError(traceError(&BucketNotFound{}), bucket, key)

}

// PutObject creates a new object with the incoming data,
func (l *pydioObjects) PutObject(bucket string, object string, size int64, data io.Reader, metadata map[string]string, sha256sum string) (objInfo ObjectInfo, e error) {
	var sha256sumBytes []byte

	var err error
	if sha256sum != "" {
		sha256sumBytes, err = hex.DecodeString(sha256sum)
		if err != nil {
			return objInfo, s3ToObjectError(traceError(err), bucket, object)
		}
	}

	var md5sumBytes []byte
	md5sum := metadata["etag"]
	if md5sum != "" {
		md5sumBytes, err = hex.DecodeString(md5sum)
		if err != nil {
			return objInfo, s3ToObjectError(traceError(err), bucket, object)
		}
		delete(metadata, "etag")
	}
	if client, ok := l.findMinioClientFor(bucket, object); ok {

		bucket, object = l.translateBucketAndPrefix(bucket, object)

		oi, err := client.PutObject(bucket, object, size, data, md5sumBytes, sha256sumBytes, toMinioClientMetadata(metadata))
		if err != nil {
			return objInfo, s3ToObjectError(traceError(err), bucket, object)
		}

		return fromMinioClientObjectInfo(bucket, oi), nil
	}
	return ObjectInfo{}, s3ToObjectError(traceError(err), bucket, object)
}

// CopyObject copies a blob from source container to destination container.
func (l *pydioObjects) CopyObject(srcBucket string, srcObject string, destBucket string, destObject string, metadata map[string]string) (objInfo ObjectInfo, e error) {

	if srcObject == destObject {
		log.Printf("Coping %v to %v, this is a REPLACE meta directive \n", srcObject, destObject)
		log.Println(metadata)
		return objInfo, traceError(&NotImplemented{})
	}
	log.Println("Received COPY instruction: ", srcBucket, "/", srcObject, "=>", destBucket, "/", destObject)

	var client *minio.Core
	var ok bool
	if client, ok = l.findMinioClientFor(srcBucket, srcObject); !ok {
		return objInfo, s3ToObjectError(traceError(&BucketNotFound{}), srcBucket, srcObject)
	}
	destBucket, destObject = l.translateBucketAndPrefix(destBucket, destObject)
	srcBucket, srcObject = l.translateBucketAndPrefix(srcBucket, srcObject)
	err := client.CopyObject(destBucket, destObject, path.Join(srcBucket, srcObject), minio.CopyConditions{})
	if err != nil {
		return objInfo, s3ToObjectError(traceError(err), srcBucket, srcObject)
	}

	oi, err := l.getS3ObjectInfo(client, destBucket, destObject)
	if err != nil {
		return objInfo, s3ToObjectError(traceError(err), destBucket, destObject)
	}

	return oi, nil
}

// DeleteObject deletes a blob in bucket
func (l *pydioObjects) DeleteObject(bucket string, object string) error {

	var client *minio.Core
	var ok bool
	if client, ok = l.findMinioClientFor(bucket, object); !ok {
		return s3ToObjectError(traceError(&BucketNotFound{}), bucket, object)
	}

	bucket, object = l.translateBucketAndPrefix(bucket, object)
	err := client.RemoveObject(bucket, object)
	if err != nil {
		return s3ToObjectError(traceError(err), bucket, object)
	}

	return nil
}

// ListMultipartUploads lists all multipart uploads.
func (l *pydioObjects) ListMultipartUploads(bucket string, prefix string, keyMarker string, uploadIDMarker string, delimiter string, maxUploads int) (lmi ListMultipartsInfo, e error) {

	var client *minio.Core
	var ok bool
	if client, ok = l.findMinioClientFor(bucket, prefix); !ok {
		return lmi, errors.New("Multipart Error")
	}

	bucket, prefix = l.translateBucketAndPrefix(bucket, prefix)
	result, err := client.ListMultipartUploads(bucket, prefix, keyMarker, uploadIDMarker, delimiter, maxUploads)
	if err != nil {
		return lmi, err
	}

	return fromMinioClientListMultipartsInfo(result), nil
}

// NewMultipartUpload upload object in multiple parts
func (l *pydioObjects) NewMultipartUpload(bucket string, object string, metadata map[string]string) (uploadID string, err error) {

	var client *minio.Core
	var ok bool
	if client, ok = l.findMinioClientFor(bucket, object); !ok {
		return "", errors.New("Multipart Error")
	}
	bucket, object = l.translateBucketAndPrefix(bucket, object)
	return client.NewMultipartUpload(bucket, object, toMinioClientMetadata(metadata))
}

// CopyObjectPart copy part of object to other bucket and object
func (l *pydioObjects) CopyObjectPart(srcBucket string, srcObject string, destBucket string, destObject string, uploadID string, partID int, startOffset int64, length int64) (info PartInfo, err error) {
	// FIXME: implement CopyObjectPart
	return PartInfo{}, traceError(NotImplemented{})
}

// PutObjectPart puts a part of object in bucket
func (l *pydioObjects) PutObjectPart(bucket string, object string, uploadID string, partID int, size int64, data io.Reader, md5Hex string, sha256sum string) (pi PartInfo, e error) {
	md5HexBytes, err := hex.DecodeString(md5Hex)
	if err != nil {
		return pi, err
	}

	sha256sumBytes, err := hex.DecodeString(sha256sum)
	if err != nil {
		return pi, err
	}

	var client *minio.Core
	var ok bool
	if client, ok = l.findMinioClientFor(bucket, object); !ok {
		return pi, errors.New("Put Object Part Error")
	}

	bucket, object = l.translateBucketAndPrefix(bucket, object)
	info, err := client.PutObjectPart(bucket, object, uploadID, partID, size, data, md5HexBytes, sha256sumBytes)
	if err != nil {
		return pi, err
	}

	return fromMinioClientObjectPart(info), nil
}

// ListObjectParts returns all object parts for specified object in specified bucket
func (l *pydioObjects) ListObjectParts(bucket string, object string, uploadID string, partNumberMarker int, maxParts int) (lpi ListPartsInfo, e error) {

	var client *minio.Core
	var ok bool
	if client, ok = l.findMinioClientFor(bucket, object); !ok {
		return lpi, errors.New("Put Object Part Error")
	}

	bucket, object = l.translateBucketAndPrefix(bucket, object)
	result, err := client.ListObjectParts(bucket, object, uploadID, partNumberMarker, maxParts)
	if err != nil {
		return lpi, err
	}

	return fromMinioClientListPartsInfo(result), nil
}

// AbortMultipartUpload aborts a ongoing multipart upload
func (l *pydioObjects) AbortMultipartUpload(bucket string, object string, uploadID string) error {
	var client *minio.Core
	var ok bool
	if client, ok = l.findMinioClientFor(bucket, object); !ok {
		return errors.New("Put Object Part Error")
	}
	bucket, object = l.translateBucketAndPrefix(bucket, object)
	return client.AbortMultipartUpload(bucket, object, uploadID)
}

// CompleteMultipartUpload completes ongoing multipart upload and finalizes object
func (l *pydioObjects) CompleteMultipartUpload(bucket string, object string, uploadID string, uploadedParts []completePart) (oi ObjectInfo, e error) {

	var client *minio.Core
	var ok bool
	if client, ok = l.findMinioClientFor(bucket, object); !ok {
		return oi, errors.New("Put Object Part Error")
	}
	bucket, object = l.translateBucketAndPrefix(bucket, object)
	err := client.CompleteMultipartUpload(bucket, object, uploadID, toMinioClientCompleteParts(uploadedParts))
	if err != nil {
		return oi, s3ToObjectError(traceError(err), bucket, object)
	}

	return l.getS3ObjectInfo(client, bucket, object)
}
