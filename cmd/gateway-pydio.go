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
	"encoding/hex"
	"io"

	"errors"
	"github.com/pydio/minio-go"
	"github.com/pydio/services/common"
	"github.com/pydio/services/common/proto/tree"
	"strings"
	"time"

	"context"
	"github.com/pydio/services/common/views"
)

type PydioGateway interface {
	GatewayLayer
	GetBucketInfoWithContext(ctx context.Context, bucket string) (bi BucketInfo, e error)
	ListBucketsWithContext(ctx context.Context) ([]BucketInfo, error)
	ListObjectsWithContext(ctx context.Context, bucket string, prefix string, marker string, delimiter string, maxKeys int) (loi ListObjectsInfo, e error)
	ListObjectsV2WithContext(ctx context.Context, bucket, prefix, continuationToken string, fetchOwner bool, delimiter string, maxKeys int) (loi ListObjectsV2Info, e error)
	GetObjectInfoWithContext(ctx context.Context, bucket string, object string) (objInfo ObjectInfo, err error)
	GetObjectWithContext(ctx context.Context, bucket string, key string, startOffset int64, length int64, writer io.Writer) error
	PutObjectWithContext(ctx context.Context, bucket string, object string, size int64, data io.Reader, metadata map[string]string, sha256sum string) (objInfo ObjectInfo, e error)
	CopyObjectWithContext(ctx context.Context, srcBucket string, srcObject string, destBucket string, destObject string, metadata map[string]string) (objInfo ObjectInfo, e error)
	DeleteObjectWithContext(ctx context.Context, bucket string, object string) error
	ListMultipartUploadsWithContext(ctx context.Context, bucket string, prefix string, keyMarker string, uploadIDMarker string, delimiter string, maxUploads int) (lmi ListMultipartsInfo, e error)
	NewMultipartUploadWithContext(ctx context.Context, bucket string, object string, metadata map[string]string) (uploadID string, err error)
	CopyObjectPartWithContext(ctx context.Context, srcBucket string, srcObject string, destBucket string, destObject string, uploadID string, partID int, startOffset int64, length int64) (info PartInfo, err error)
	PutObjectPartWithContext(ctx context.Context, bucket string, object string, uploadID string, partID int, size int64, data io.Reader, md5Hex string, sha256sum string) (pi PartInfo, e error)
	ListObjectPartsWithContext(ctx context.Context, bucket string, object string, uploadID string, partNumberMarker int, maxParts int) (lpi ListPartsInfo, e error)
	AbortMultipartUploadWithContext(ctx context.Context, bucket string, object string, uploadID string) error
	CompleteMultipartUploadWithContext(ctx context.Context, bucket string, object string, uploadID string, uploadedParts []completePart) (oi ObjectInfo, e error)
}

// BucketNotFound bucket does not exist.
type ContextNotFound GenericError

func (e ContextNotFound) Error() string {
	return "Context not found. Use WithContext function."
}

// s3Objects implements gateway for Minio and S3 compatible object storage servers.
type pydioObjects struct {
	Router *views.Router
}

// newS3Gateway returns s3 gatewaylayer
func newPydioGateway() (GatewayLayer, error) {

	pool := views.NewClientsPool()
	router := views.NewRouter(pool, []views.Handler{
		&views.ThumbsHandler{
			StoreName: common.PYDIO_THUMBSTORE_NAMESPACE, // Direct access to dedicated Bucket for thumbnails
		},
		&views.ArchiveHandler{},    // Catch "GET" request on folder.zip and create archive on-demand
		&views.ViewAdmin{},         // Currently merge TreeClient & S3 Clients to display whole tree
		&views.EncryptionHandler{}, // Handler retrieve encryption materials from encryption service
		&views.MkfileHandler{},     // Handler adding a node precreation on PUT file request
		&views.Executor{},          // Final handler using the passed data to perform actual tree/s3 queries
	})

	api := &pydioObjects{
		Router: router,
	}

	return api, nil
}

// fromMinioClientObjectInfo converts minio ObjectInfo to gateway ObjectInfo
func fromPydioNodeObjectInfo(bucket string, node *tree.Node) ObjectInfo {

	cType := "application/octet-stream"
	userDefined := map[string]string{
		"Content-Type": cType,
	}

	nodePath := node.Path
	if node.Type == tree.NodeType_COLLECTION {
		nodePath += "/"
	}
	return ObjectInfo{
		Bucket:          bucket,
		Name:            nodePath,
		ModTime:         time.Unix(0, node.MTime*int64(time.Second)),
		Size:            node.Size,
		ETag:            canonicalizeETag(node.Etag),
		UserDefined:     userDefined,
		ContentType:     cType,
		ContentEncoding: "",
	}
}

func (l *pydioObjects) ListPydioObjects(ctx context.Context, bucket string, prefix string, delimiter string, maxKeys int) (objects []ObjectInfo, prefixes []string, err error) {

	treePath := strings.TrimLeft(prefix, "/")
	recursive := false
	if delimiter == "" {
		recursive = true
	}
	var FilterType tree.NodeType
	if maxKeys == 1 {
		// We probably want to get only the very first object here (for folders stats)
		log.Println("Should get only LEAF nodes")
		FilterType = tree.NodeType_LEAF
	}
	lNodeClient, err := l.Router.ListNodes(ctx, &tree.ListNodesRequest{
		Node: &tree.Node{
			Path: treePath,
		},
		Recursive:  recursive,
		Limit:      int64(maxKeys),
		FilterType: FilterType,
	})
	if err != nil {
		return nil, nil, s3ToObjectError(traceError(err), bucket)
	}
	for {
		clientResponse, err := lNodeClient.Recv()
		if err != nil {
			break
		}
		if clientResponse == nil {
			continue
		}
		objectInfo := fromPydioNodeObjectInfo(bucket, clientResponse.Node)
		if clientResponse.Node.IsLeaf() {
			objects = append(objects, objectInfo)
		} else {
			prefixes = append(prefixes, objectInfo.Name)
		}

	}
	if len(objects) > 0 && strings.Trim(prefix, "/") != "" {
		prefixes = append(prefixes, strings.TrimLeft(prefix, "/"))
	}

	return objects, prefixes, nil
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
func (l *pydioObjects) GetBucketInfoWithContext(ctx context.Context, bucket string) (bi BucketInfo, e error) {

	if bucket != "pydio" {
		return bi, traceError(BucketNotFound{Bucket: bucket})
	}
	return BucketInfo{
		Name:    bucket,
		Created: time.Now(),
	}, nil

}

// ListBuckets lists all S3 buckets
func (l *pydioObjects) ListBucketsWithContext(ctx context.Context) ([]BucketInfo, error) {

	b := make([]BucketInfo, 1)
	b[0] = BucketInfo{
		Name:    "pydio",
		Created: time.Now(),
	}
	return b, nil

}

// ListObjects lists all blobs in S3 bucket filtered by prefix
func (l *pydioObjects) ListObjectsWithContext(ctx context.Context, bucket string, prefix string, marker string, delimiter string, maxKeys int) (loi ListObjectsInfo, e error) {

	objects, prefixes, err := l.ListPydioObjects(ctx, bucket, prefix, delimiter, maxKeys)
	if err != nil {
		return loi, s3ToObjectError(traceError(err), bucket)
	}

	log.Printf("[ListObjects] Returning %d objects and %d prefixes (V1) for prefix\n", len(objects), len(prefixes), prefix)

	return ListObjectsInfo{
		IsTruncated: false,
		NextMarker:  "",
		Prefixes:    prefixes,
		Objects:     objects,
	}, nil

}

// ListObjectsV2 lists all blobs in S3 bucket filtered by prefix
func (l *pydioObjects) ListObjectsV2WithContext(ctx context.Context, bucket, prefix, continuationToken string, fetchOwner bool, delimiter string, maxKeys int) (loi ListObjectsV2Info, e error) {

	objects, prefixes, err := l.ListPydioObjects(ctx, bucket, prefix, delimiter, maxKeys)
	if err != nil {
		return loi, s3ToObjectError(traceError(err), bucket)
	}

	log.Printf("\n[ListObjectsV2] Returning %d objects and %d prefixes (V2) for prefix\n", len(objects), len(prefixes), prefix)

	return ListObjectsV2Info{
		IsTruncated: false,
		Prefixes:    prefixes,
		Objects:     objects,

		ContinuationToken:     "",
		NextContinuationToken: "",
	}, nil

}

// GetObjectInfo reads object info and replies back ObjectInfo
func (l *pydioObjects) GetObjectInfoWithContext(ctx context.Context, bucket string, object string) (objInfo ObjectInfo, err error) {

	log.Println("[GetObjectInfo]" + object)

	path := strings.TrimLeft(object, "/")
	readNodeResponse, err := l.Router.ReadNode(ctx, &tree.ReadNodeRequest{Node: &tree.Node{
		Path: path,
	}})
	if err != nil {
		return ObjectInfo{}, s3ToObjectError(traceError(err), bucket, object)
	}

	if !readNodeResponse.Node.IsLeaf() {
		return ObjectInfo{}, errors.New("S3 API Cannot send object info for folder")
	}

	return fromPydioNodeObjectInfo(bucket, readNodeResponse.Node), nil

}

// GetObject reads an object from S3. Supports additional
// parameters like offset and length which are synonymous with
// HTTP Range requests.
//
// startOffset indicates the starting read location of the object.
// length indicates the total length of the object.
func (l *pydioObjects) GetObjectWithContext(ctx context.Context, bucket string, key string, startOffset int64, length int64, writer io.Writer) error {

	log.Println("[GetObject] From Router", bucket, key, startOffset, length)

	path := strings.TrimLeft(key, "/")
	objectReader, err := l.Router.GetObject(ctx, &tree.Node{
		Path: path,
	}, &views.GetRequestData{
		StartOffset: startOffset,
		Length:      length,
	})
	if err != nil {
		return s3ToObjectError(traceError(err), bucket, key)
	}
	defer objectReader.Close()
	if _, err := io.Copy(writer, objectReader); err != nil {
		return s3ToObjectError(traceError(err), bucket, key)
	}
	return nil

}

// PutObject creates a new object with the incoming data,
func (l *pydioObjects) PutObjectWithContext(ctx context.Context, bucket string, object string, size int64, data io.Reader, requestMetadata map[string]string, sha256sum string) (objInfo ObjectInfo, e error) {

	var sha256sumBytes []byte
	var md5sumBytes []byte

	var err error
	if sha256sum != "" {
		sha256sumBytes, err = hex.DecodeString(sha256sum)
		if err != nil {
			return objInfo, s3ToObjectError(traceError(err), bucket, object)
		}
	}

	md5sum := requestMetadata["etag"]
	if md5sum != "" {
		md5sumBytes, err = hex.DecodeString(md5sum)
		if err != nil {
			return objInfo, s3ToObjectError(traceError(err), bucket, object)
		}
		delete(requestMetadata, "etag")
	}

	written, err := l.Router.PutObject(ctx, &tree.Node{
		Path: strings.TrimLeft(object, "/"),
	}, data, &views.PutRequestData{
		Size:      size,
		Sha256Sum: sha256sumBytes,
		Md5Sum:    md5sumBytes,
	})
	if err != nil {
		return objInfo, s3ToObjectError(traceError(err), bucket, object)
	}
	// TODO : PutObject should return more info about written node
	objInfo = ObjectInfo{
		Bucket: bucket,
		Name:   object,
		Size:   written,
	}
	return objInfo, nil

}

// CopyObject copies a blob from source container to destination container.
func (l *pydioObjects) CopyObjectWithContext(ctx context.Context, srcBucket string, srcObject string, destBucket string, destObject string, requestMetadata map[string]string) (objInfo ObjectInfo, e error) {

	if srcObject == destObject {
		log.Printf("Coping %v to %v, this is a REPLACE meta directive \n", srcObject, destObject)
		log.Println(requestMetadata)
		return objInfo, traceError(&NotImplemented{})
	}
	log.Println("Received COPY instruction: ", srcBucket, "/", srcObject, "=>", destBucket, "/", destObject)

	written, err := l.Router.CopyObject(ctx, &tree.Node{
		Path: strings.TrimLeft(srcObject, "/"),
	}, &tree.Node{
		Path: strings.TrimLeft(destObject, "/"),
	}, &views.CopyRequestData{})

	if err != nil {
		return objInfo, s3ToObjectError(traceError(&BucketNotFound{}), srcBucket, srcObject)
	}
	return ObjectInfo{
		Bucket: destBucket,
		Name:   destObject,
		Size:   written,
	}, nil

}

// DeleteObject deletes a blob in bucket
func (l *pydioObjects) DeleteObjectWithContext(ctx context.Context, bucket string, object string) error {

	log.Println("[DeleteObject]", object)
	_, err := l.Router.DeleteNode(ctx, &tree.DeleteNodeRequest{
		Node: &tree.Node{
			Path: strings.TrimLeft(object, "/"),
		},
	})
	if err != nil {
		return s3ToObjectError(traceError(err), bucket, object)
	}
	return nil

}

// ListMultipartUploads lists all multipart uploads.
func (l *pydioObjects) ListMultipartUploadsWithContext(ctx context.Context, bucket string, prefix string, keyMarker string, uploadIDMarker string, delimiter string, maxUploads int) (lmi ListMultipartsInfo, e error) {

	result, err := l.Router.MultipartList(ctx, prefix, &views.MultipartRequestData{
		ListKeyMarker:      keyMarker,
		ListUploadIDMarker: uploadIDMarker,
		ListDelimiter:      delimiter,
		ListMaxUploads:     maxUploads,
	})
	if err == nil {
		return fromMinioClientListMultipartsInfo(result), nil
	} else {
		return lmi, err
	}

}

// NewMultipartUpload upload object in multiple parts
func (l *pydioObjects) NewMultipartUploadWithContext(ctx context.Context, bucket string, object string, reqMetadata map[string]string) (uploadID string, err error) {

	return l.Router.MultipartCreate(ctx, &tree.Node{
		Path: object,
	}, &views.MultipartRequestData{
		Metadata: toMinioClientMetadata(reqMetadata),
	})

}

// PutObjectPart puts a part of object in bucket
func (l *pydioObjects) PutObjectPartWithContext(ctx context.Context, bucket string, object string, uploadID string, partID int, size int64, data io.Reader, md5Hex string, sha256sum string) (pi PartInfo, e error) {

	md5HexBytes, err := hex.DecodeString(md5Hex)
	if err != nil {
		return pi, err
	}

	sha256sumBytes, err := hex.DecodeString(sha256sum)
	if err != nil {
		return pi, err
	}
	written, err := l.Router.PutObject(ctx, &tree.Node{Path: object}, data, &views.PutRequestData{
		Size:              size,
		Md5Sum:            md5HexBytes,
		Sha256Sum:         sha256sumBytes,
		MultipartPartID:   partID,
		MultipartUploadID: uploadID,
	})
	if err != nil {
		return pi, err
	}
	// TODO
	//return fromMinioClientObjectPart(info), nil
	return PartInfo{
		Size:         written,
		ETag:         "",
		LastModified: time.Now(),
		PartNumber:   partID,
	}, nil

}

// ListObjectParts returns all object parts for specified object in specified bucket
func (l *pydioObjects) ListObjectPartsWithContext(ctx context.Context, bucket string, object string, uploadID string, partNumberMarker int, maxParts int) (lpi ListPartsInfo, e error) {

	result, err := l.Router.MultipartListObjectParts(ctx, &tree.Node{Path: object}, uploadID, partNumberMarker, maxParts)
	if err != nil {
		return lpi, err
	}
	return fromMinioClientListPartsInfo(result), nil

}

// AbortMultipartUpload aborts a ongoing multipart upload
func (l *pydioObjects) AbortMultipartUploadWithContext(ctx context.Context, bucket string, object string, uploadID string) error {

	return l.Router.MultipartAbort(ctx, &tree.Node{Path: object}, uploadID, &views.MultipartRequestData{})

}

// CompleteMultipartUpload completes ongoing multipart upload and finalizes object
func (l *pydioObjects) CompleteMultipartUploadWithContext(ctx context.Context, bucket string, object string, uploadID string, uploadedParts []completePart) (oi ObjectInfo, e error) {

	out, err := l.Router.MultipartComplete(ctx, &tree.Node{Path: object}, uploadID, toMinioClientCompleteParts(uploadedParts))
	return fromMinioClientObjectInfo(bucket, out), err

}

//////// UTILS ////////

// GetObjectInfo reads object info and replies back ObjectInfo
func (l *pydioObjects) getS3ObjectInfo(client *minio.Core, bucket string, object string) (objInfo ObjectInfo, err error) {
	r := minio.NewHeadReqHeaders()
	oi, err := client.StatObject(bucket, object, r)
	if err != nil {
		return ObjectInfo{}, s3ToObjectError(traceError(err), bucket, object)
	}

	return fromMinioClientObjectInfo(bucket, oi), nil
}

/////// ORIGINAL METHODS WITHOUT CONTEXT ////////

// GetBucketInfo gets bucket metadata..
func (l *pydioObjects) GetBucketInfo(bucket string) (bi BucketInfo, e error) {
	return bi, traceError(ContextNotFound{Bucket: bucket})
}

// ListBuckets lists all S3 buckets
func (l *pydioObjects) ListBuckets() (bi []BucketInfo, e error) {
	return bi, traceError(ContextNotFound{})
}

// ListObjects lists all blobs in S3 bucket filtered by prefix
func (l *pydioObjects) ListObjects(bucket string, prefix string, marker string, delimiter string, maxKeys int) (loi ListObjectsInfo, e error) {
	return loi, traceError(ContextNotFound{Bucket: bucket, Object: prefix})
}

// ListObjectsV2 lists all blobs in S3 bucket filtered by prefix
func (l *pydioObjects) ListObjectsV2(bucket, prefix, continuationToken string, fetchOwner bool, delimiter string, maxKeys int) (loi ListObjectsV2Info, e error) {
	return loi, traceError(ContextNotFound{Bucket: bucket, Object: prefix})
}

// GetObjectInfo reads object info and replies back ObjectInfo
func (l *pydioObjects) GetObjectInfo(bucket string, object string) (objInfo ObjectInfo, err error) {
	return objInfo, traceError(ContextNotFound{Bucket: bucket, Object: object})
}

// GetObject reads an object from S3. Supports additional
// parameters like offset and length which are synonymous with
// HTTP Range requests.
//
// startOffset indicates the starting read location of the object.
// length indicates the total length of the object.
func (l *pydioObjects) GetObject(bucket string, key string, startOffset int64, length int64, writer io.Writer) error {
	return traceError(ContextNotFound{Bucket: bucket, Object: key})
}

// PutObject creates a new object with the incoming data,
func (l *pydioObjects) PutObject(bucket string, object string, size int64, data io.Reader, metadata map[string]string, sha256sum string) (objInfo ObjectInfo, e error) {
	return objInfo, traceError(ContextNotFound{Bucket: bucket, Object: object})
}

// CopyObject copies a blob from source container to destination container.
func (l *pydioObjects) CopyObject(srcBucket string, srcObject string, destBucket string, destObject string, metadata map[string]string) (objInfo ObjectInfo, e error) {
	return objInfo, traceError(ContextNotFound{Bucket: srcBucket, Object: srcObject})
}

// DeleteObject deletes a blob in bucket
func (l *pydioObjects) DeleteObject(bucket string, object string) error {
	return traceError(ContextNotFound{Bucket: bucket, Object: object})
}

// ListMultipartUploads lists all multipart uploads.
func (l *pydioObjects) ListMultipartUploads(bucket string, prefix string, keyMarker string, uploadIDMarker string, delimiter string, maxUploads int) (lmi ListMultipartsInfo, e error) {
	return lmi, traceError(ContextNotFound{Bucket: bucket, Object: prefix})
}

// NewMultipartUpload upload object in multiple parts
func (l *pydioObjects) NewMultipartUpload(bucket string, object string, metadata map[string]string) (uploadID string, err error) {
	return uploadID, traceError(ContextNotFound{Bucket: bucket, Object: object})
}

// CopyObjectPart copy part of object to other bucket and object
func (l *pydioObjects) CopyObjectPartWithContext(ctx context.Context, srcBucket string, srcObject string, destBucket string, destObject string, uploadID string, partID int, startOffset int64, length int64) (info PartInfo, err error) {
	return l.CopyObjectPart(srcBucket, srcObject, destBucket, destObject, uploadID, partID, startOffset, length)
}

// CopyObjectPart copy part of object to other bucket and object
func (l *pydioObjects) CopyObjectPart(srcBucket string, srcObject string, destBucket string, destObject string, uploadID string, partID int, startOffset int64, length int64) (info PartInfo, err error) {
	// FIXME: implement CopyObjectPart
	return PartInfo{}, traceError(NotImplemented{})
}

// PutObjectPart puts a part of object in bucket
func (l *pydioObjects) PutObjectPart(bucket string, object string, uploadID string, partID int, size int64, data io.Reader, md5Hex string, sha256sum string) (pi PartInfo, e error) {
	return pi, traceError(ContextNotFound{Bucket: bucket, Object: object})
}

// ListObjectParts returns all object parts for specified object in specified bucket
func (l *pydioObjects) ListObjectParts(bucket string, object string, uploadID string, partNumberMarker int, maxParts int) (lpi ListPartsInfo, e error) {
	return lpi, traceError(ContextNotFound{Bucket: bucket, Object: object})
}

// AbortMultipartUpload aborts a ongoing multipart upload
func (l *pydioObjects) AbortMultipartUpload(bucket string, object string, uploadID string) error {
	return traceError(ContextNotFound{Bucket: bucket, Object: object})
}

// CompleteMultipartUpload completes ongoing multipart upload and finalizes object
func (l *pydioObjects) CompleteMultipartUpload(bucket string, object string, uploadID string, uploadedParts []completePart) (oi ObjectInfo, e error) {
	return oi, traceError(ContextNotFound{Bucket: bucket, Object: object})
}
