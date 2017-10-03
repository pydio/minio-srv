package views

import (
	"io"
	"strings"
	"time"

	"github.com/pydio/services/common/log"

	"github.com/micro/go-micro/client"
	"github.com/micro/go-micro/errors"
	"github.com/micro/go-micro/metadata"
	"github.com/pydio/minio-go"
	"github.com/pydio/services/common"
	"github.com/pydio/services/common/proto/tree"
	uuid2 "github.com/satori/go.uuid"
	"go.uber.org/zap"
	"golang.org/x/net/context"
)

type Executor struct {
	AbstractHandler
}

func (a *Executor) ExecuteWrapped(inputFilter NodeFilter, outputFilter NodeFilter, provider NodesCallback) error {

	return provider(inputFilter, outputFilter)

}

func (e *Executor) ReadNode(ctx context.Context, in *tree.ReadNodeRequest, opts ...client.CallOption) (*tree.ReadNodeResponse, error) {
	reader, _ := TreeClientsFromContext(ctx)
	if reader == nil {
		return nil, errors.BadRequest(VIEWS_LIBRARY_NAME, "Cannot find tree client, did you insert a resolver middleware?")
	}
	return reader.ReadNode(ctx, in, opts...)
}

func (e *Executor) ListNodes(ctx context.Context, in *tree.ListNodesRequest, opts ...client.CallOption) (tree.NodeProvider_ListNodesClient, error) {
	reader, _ := TreeClientsFromContext(ctx)
	if reader == nil {
		return nil, errors.BadRequest(VIEWS_LIBRARY_NAME, "Cannot find tree client, did you insert a resolver middleware?")
	}
	return reader.ListNodes(ctx, in, opts...)
}

func (e *Executor) CreateNode(ctx context.Context, in *tree.CreateNodeRequest, opts ...client.CallOption) (*tree.CreateNodeResponse, error) {
	_, writer := TreeClientsFromContext(ctx)
	if writer == nil {
		return nil, errors.BadRequest(VIEWS_LIBRARY_NAME, "Cannot find tree client, did you insert a resolver middleware?")
	}
	node := in.Node
	if !node.IsLeaf() {
		log.Logger(ctx).Info("CREATE / Should Create A Hidden .__pydio", zap.String("path", node.Path))
		// This should be in fact a PutObject
		uuid := uuid2.NewV4().String()
		dsPath := node.GetStringMeta(common.META_NAMESPACE_DATASOURCE_PATH)
		newNode := &tree.Node{
			Path: strings.TrimRight(node.Path, "/") + "/.__pydio",
		}
		newNode.SetMeta(common.META_NAMESPACE_DATASOURCE_PATH, dsPath+"/.__pydio")
		_, err := e.PutObject(ctx, newNode, strings.NewReader(uuid), &PutRequestData{Size: int64(len(uuid))})
		if err != nil {
			return nil, err
		}
		node.Uuid = uuid
		node.MTime = time.Now().Unix()
		return &tree.CreateNodeResponse{Node: node}, nil
	}
	return writer.CreateNode(ctx, in, opts...)
}

func (e *Executor) UpdateNode(ctx context.Context, in *tree.UpdateNodeRequest, opts ...client.CallOption) (*tree.UpdateNodeResponse, error) {
	// Should probably not be called directly. Handle cases where DS are the same or not.
	_, writer := TreeClientsFromContext(ctx)
	if writer == nil {
		return nil, errors.BadRequest(VIEWS_LIBRARY_NAME, "Cannot find tree client, did you insert a resolver middleware?")
	}
	return writer.UpdateNode(ctx, in, opts...)
}

func (e *Executor) DeleteNode(ctx context.Context, in *tree.DeleteNodeRequest, opts ...client.CallOption) (*tree.DeleteNodeResponse, error) {
	info, ok := GetBranchInfo(ctx, "in")
	if !ok {
		return nil, errors.BadRequest(VIEWS_LIBRARY_NAME, "Cannot find S3 client, did you insert a resolver middleware?")
	}
	writer := info.Client
	if meta, mOk := metadata.FromContext(ctx); mOk {
		writer.PrepareMetadata(meta)
		defer writer.ClearMetadata()
	}

	err := writer.RemoveObject(info.Bucket, in.Node.GetStringMeta(common.META_NAMESPACE_DATASOURCE_PATH))
	success := true
	if err != nil {
		success = false
	}
	return &tree.DeleteNodeResponse{Success: success}, err
}

func (e *Executor) GetObject(ctx context.Context, node *tree.Node, requestData *GetRequestData) (io.ReadCloser, error) {
	info, ok := GetBranchInfo(ctx, "in")
	if !ok {
		return nil, errors.BadRequest(VIEWS_LIBRARY_NAME, "Cannot find S3 client, did you insert a resolver middleware?")
	}
	writer := info.Client
	if meta, mOk := metadata.FromContext(ctx); mOk {
		log.Logger(ctx).Debug("Preparing Meta for GetObject", zap.Any("meta", meta))
		writer.PrepareMetadata(meta)
		defer writer.ClearMetadata()
	} else {
		log.Logger(ctx).Debug("Preparing Meta for GetObject: No Meta Found")
	}

	var reader io.ReadCloser
	var err error

	s3Path := node.GetStringMeta(common.META_NAMESPACE_DATASOURCE_PATH)
	if requestData.EncryptionMaterial != nil {
		reader, err = writer.GetEncryptedObject(info.Bucket, s3Path, requestData.EncryptionMaterial)
	} else {
		headers := minio.GetObjectOptions{}
		if requestData.StartOffset >= 0 && requestData.Length >= 0 {
			if err := headers.SetRange(requestData.StartOffset, requestData.StartOffset+requestData.Length-1); err != nil {
				return nil, err
			}
		}
		log.Logger(ctx).Debug("Get Object", zap.String("bucket", info.Bucket), zap.String("s3path", s3Path), zap.Any("headers", headers), zap.Any("request", requestData))
		reader, _, err = writer.GetObject(info.Bucket, s3Path, headers)
		if err != nil {
			log.Logger(ctx).Debug("Get Object", zap.Error(err))
		}
	}
	return reader, err
}

func (e *Executor) PutObject(ctx context.Context, node *tree.Node, reader io.Reader, requestData *PutRequestData) (int64, error) {
	info, ok := GetBranchInfo(ctx, "in")
	if !ok {
		return 0, errors.BadRequest(VIEWS_LIBRARY_NAME, "Cannot find S3 client, did you insert a resolver middleware?")
	}
	writer := info.Client
	if meta, mOk := metadata.FromContext(ctx); mOk {
		writer.PrepareMetadata(meta)
		defer writer.ClearMetadata()
	}
	s3Path := node.GetStringMeta(common.META_NAMESPACE_DATASOURCE_PATH)

	if requestData.EncryptionMaterial != nil {

		return writer.PutEncryptedObject(info.Bucket, s3Path, reader, requestData.EncryptionMaterial)

	} else {
		oi, err := writer.PutObject(info.Bucket, s3Path, reader, requestData.Size, requestData.Md5Sum, requestData.Sha256Sum, nil)
		if err != nil {
			return 0, err
		} else {
			return oi.Size, nil
		}

	}

}

func (e *Executor) CopyObject(ctx context.Context, from *tree.Node, to *tree.Node, requestData *CopyRequestData) (int64, error) {

	// If DS's are same datasource, simple S3 Copy operation. Otherwise it must copy from one to another.
	destInfo, ok := GetBranchInfo(ctx, "to")
	srcInfo, ok2 := GetBranchInfo(ctx, "from")
	if !ok || !ok2 {
		return 0, errors.InternalServerError(VIEWS_LIBRARY_NAME, "Cannot find DSInfo client for src or dest")
	}
	destClient := destInfo.Client
	srcClient := srcInfo.Client
	destBucket := destInfo.Bucket
	srcBucket := srcInfo.Bucket
	if meta, mOk := metadata.FromContext(ctx); mOk {
		destClient.PrepareMetadata(meta)
		srcClient.PrepareMetadata(meta)
		defer srcClient.ClearMetadata()
		defer destClient.ClearMetadata()
	}

	// var srcSse, destSse minio.SSEInfo
	// if requestData.srcEncryptionMaterial != nil {
	// 	srcSse = minio.NewSSEInfo([]byte(requestData.srcEncryptionMaterial.GetKey()), "")
	// }
	// if requestData.destEncryptionMaterial != nil {
	// 	destSse = minio.NewSSEInfo([]byte(requestData.destEncryptionMaterial.GetKey()), "")
	// }

	fromPath := from.GetStringMeta(common.META_NAMESPACE_DATASOURCE_PATH)
	toPath := to.GetStringMeta(common.META_NAMESPACE_DATASOURCE_PATH)

	if destClient == srcClient {

		meta := make(map[string]string, len(requestData.Metadata))
		for k, v := range requestData.Metadata {
			meta[k] = strings.Join(v, "")
		}
		// srcInfo := minio.NewSourceInfo(srcBucket, fromPath, &srcSse)
		// destInfo, err := minio.NewDestinationInfo(destBucket, toPath, &destSse, meta)
		// if err != nil {
		// 	return 0, err
		// }
		oi, err := destClient.CopyObject(srcBucket, fromPath, destBucket, toPath, nil)
		if err != nil {
			return 0, err
		}
		// oi, err3 := destClient.StatObject(destBucket, toPath, minio.NewHeadReqHeaders())
		// if err3 != nil {
		// 	return 0, err3
		// }
		return oi.Size, nil

	} else {

		var reader io.ReadCloser
		var err error
		srcInfo, srcErr := srcClient.StatObject(srcBucket, fromPath, minio.StatObjectOptions{})
		if srcErr != nil {
			return 0, srcErr
		}
		if requestData.srcEncryptionMaterial != nil {
			reader, err = srcClient.GetEncryptedObject(srcBucket, fromPath, requestData.srcEncryptionMaterial)
		} else {
			reader, _, err = srcClient.GetObject(srcBucket, fromPath, minio.GetObjectOptions{})
		}
		if err != nil {
			return 0, err
		}

		if requestData.destEncryptionMaterial != nil {
			return destClient.PutEncryptedObject(destBucket, toPath, reader, requestData.destEncryptionMaterial)
		} else {
			oi, err := destClient.PutObject(destBucket, toPath, reader, srcInfo.Size, nil, nil, nil)
			if err != nil {
				log.Logger(ctx).Error("CopyObject / Different Clients", zap.Error(err), zap.Any("src", srcInfo), zap.Any("destBucket", destBucket), zap.Any("to", toPath))
			}
			return oi.Size, err
		}

	}

}

func (e *Executor) MultipartCreate(ctx context.Context, target *tree.Node, requestData *MultipartRequestData) (string, error) {

	info, ok := GetBranchInfo(ctx, "in")
	if !ok {
		return "", errors.InternalServerError(VIEWS_LIBRARY_NAME, "Cannot find client")
	}
	if meta, mOk := metadata.FromContext(ctx); mOk {
		info.Client.PrepareMetadata(meta)
		defer info.Client.ClearMetadata()
	}
	return info.Client.NewMultipartUpload(info.Bucket, target.GetStringMeta(common.META_NAMESPACE_DATASOURCE_PATH), minio.PutObjectOptions{})
}

func (e *Executor) MultipartList(ctx context.Context, prefix string, requestData *MultipartRequestData) (res minio.ListMultipartUploadsResult, err error) {
	info, ok := GetBranchInfo(ctx, "in")
	if !ok {
		return res, errors.InternalServerError(VIEWS_LIBRARY_NAME, "Cannot find client")
	}
	if meta, mOk := metadata.FromContext(ctx); mOk {
		info.Client.PrepareMetadata(meta)
		defer info.Client.ClearMetadata()
	}
	return info.Client.ListMultipartUploads(info.Bucket, prefix, requestData.ListKeyMarker, requestData.ListUploadIDMarker, requestData.ListDelimiter, requestData.ListMaxUploads)
}
func (e *Executor) MultipartAbort(ctx context.Context, target *tree.Node, uploadID string, requestData *MultipartRequestData) error {
	info, ok := GetBranchInfo(ctx, "in")
	if !ok {
		return errors.InternalServerError(VIEWS_LIBRARY_NAME, "Cannot find client")
	}
	if meta, mOk := metadata.FromContext(ctx); mOk {
		info.Client.PrepareMetadata(meta)
		defer info.Client.ClearMetadata()
	}
	return info.Client.AbortMultipartUpload(info.Bucket, target.GetStringMeta(common.META_NAMESPACE_DATASOURCE_PATH), uploadID)
}
func (e *Executor) MultipartComplete(ctx context.Context, target *tree.Node, uploadID string, uploadedParts []minio.CompletePart) (minio.ObjectInfo, error) {
	info, ok := GetBranchInfo(ctx, "in")
	if !ok {
		return minio.ObjectInfo{}, errors.InternalServerError(VIEWS_LIBRARY_NAME, "Cannot find client")
	}
	if meta, mOk := metadata.FromContext(ctx); mOk {
		info.Client.PrepareMetadata(meta)
		defer info.Client.ClearMetadata()
	}
	err := info.Client.CompleteMultipartUpload(info.Bucket, target.GetStringMeta(common.META_NAMESPACE_DATASOURCE_PATH), uploadID, uploadedParts)
	if err != nil {
		return minio.ObjectInfo{}, err
	}
	return info.Client.StatObject(info.Bucket, target.GetStringMeta(common.META_NAMESPACE_DATASOURCE_PATH), minio.StatObjectOptions{})
}
func (e *Executor) MultipartListObjectParts(ctx context.Context, target *tree.Node, uploadID string, partNumberMarker int, maxParts int) (lpi minio.ListObjectPartsResult, err error) {
	info, ok := GetBranchInfo(ctx, "in")
	if !ok {
		return lpi, errors.InternalServerError(VIEWS_LIBRARY_NAME, "Cannot find client")
	}
	if meta, mOk := metadata.FromContext(ctx); mOk {
		info.Client.PrepareMetadata(meta)
		defer info.Client.ClearMetadata()
	}
	return info.Client.ListObjectParts(info.Bucket, target.GetStringMeta(common.META_NAMESPACE_DATASOURCE_PATH), uploadID, partNumberMarker, maxParts)
}
