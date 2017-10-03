package views

import (
	"io"
	"path"
	"strings"

	"github.com/micro/go-micro/client"
	"github.com/micro/go-micro/errors"
	"github.com/micro/go-micro/metadata"
	"github.com/pydio/minio-go"
	"github.com/pydio/services/common"
	"github.com/pydio/services/common/proto/tree"
	"golang.org/x/net/context"
)

type BinaryStoreHandler struct {
	AbstractHandler
	StoreName     string
	AllowPut      bool
	AllowAnonRead bool
}

func (a *BinaryStoreHandler) isStorePath(nodePath string) bool {
	parts := strings.Split(strings.Trim(nodePath, "/"), "/")
	return len(parts) > 0 && parts[0] == a.StoreName
}

func (a *BinaryStoreHandler) updateContextForAnonRead(ctx context.Context) context.Context {
	if u := ctx.Value(common.PYDIO_CONTEXT_USER_KEY); u == nil && a.AllowAnonRead {
		ctx = context.WithValue(ctx, common.PYDIO_CONTEXT_USER_KEY, "anonymous")
		ctx = metadata.NewContext(ctx, metadata.Metadata{common.PYDIO_CONTEXT_USER_KEY: "anonymous"})
	}
	return ctx
}

// Listing of Thumbs Store : do not display content
func (a *BinaryStoreHandler) ListNodes(ctx context.Context, in *tree.ListNodesRequest, opts ...client.CallOption) (c tree.NodeProvider_ListNodesClient, e error) {
	if a.isStorePath(in.Node.Path) {
		emptyStreamer := NewWrappingStreamer()
		emptyStreamer.Close()
		return emptyStreamer, nil
	}
	return a.next.ListNodes(ctx, in, opts...)
}

// Node Info & Node Content : send by UUID,
// TODO check node rights in the ACL ?
func (a *BinaryStoreHandler) ReadNode(ctx context.Context, in *tree.ReadNodeRequest, opts ...client.CallOption) (*tree.ReadNodeResponse, error) {
	if a.isStorePath(in.Node.Path) {
		dsInfo, er := a.clientsPool.GetDataSourceInfo(a.StoreName)
		if er != nil {
			return nil, er
		}
		ctx = a.updateContextForAnonRead(ctx)
		s3client := dsInfo.Client
		if meta, mOk := metadata.FromContext(ctx); mOk {
			s3client.PrepareMetadata(meta)
			defer s3client.ClearMetadata()
		}
		objectInfo, err := s3client.StatObject(dsInfo.Bucket, path.Base(in.Node.Path), minio.StatObjectOptions{})
		if err != nil {
			return nil, err
		}
		node := &tree.Node{
			Path:  a.StoreName + "/" + objectInfo.Key,
			Size:  objectInfo.Size,
			MTime: objectInfo.LastModified.Unix(),
			Etag:  objectInfo.ETag,
			Type:  tree.NodeType_LEAF,
			Uuid:  objectInfo.Key,
			Mode:  0777,
		}
		return &tree.ReadNodeResponse{
			Node: node,
		}, nil

	}
	return a.next.ReadNode(ctx, in, opts...)
}

// TODO check node rights in the ACL ?
func (a *BinaryStoreHandler) GetObject(ctx context.Context, node *tree.Node, requestData *GetRequestData) (io.ReadCloser, error) {
	if a.isStorePath(node.Path) {
		dsInfo, er := a.clientsPool.GetDataSourceInfo(a.StoreName)
		ctx = a.updateContextForAnonRead(ctx)
		if er == nil {
			ctx = WithBranchInfo(ctx, "in", BranchInfo{DSInfo: dsInfo})
			node.SetMeta(common.META_NAMESPACE_DATASOURCE_PATH, path.Base(node.Path))
		}
	}
	return a.next.GetObject(ctx, node, requestData)
}

///////////////////////////////
// THIS STORE IS NOT WRITEABLE
///////////////////////////////
func (a *BinaryStoreHandler) CreateNode(ctx context.Context, in *tree.CreateNodeRequest, opts ...client.CallOption) (*tree.CreateNodeResponse, error) {
	if a.isStorePath(in.Node.Path) {
		return nil, errors.Forbidden(VIEWS_LIBRARY_NAME, "Forbidden store")
	}
	return a.next.CreateNode(ctx, in, opts...)
}

func (a *BinaryStoreHandler) UpdateNode(ctx context.Context, in *tree.UpdateNodeRequest, opts ...client.CallOption) (*tree.UpdateNodeResponse, error) {
	if a.isStorePath(in.From.Path) || a.isStorePath(in.To.Path) {
		return nil, errors.Forbidden(VIEWS_LIBRARY_NAME, "Forbidden store")
	}
	return a.next.UpdateNode(ctx, in, opts...)
}

func (a *BinaryStoreHandler) DeleteNode(ctx context.Context, in *tree.DeleteNodeRequest, opts ...client.CallOption) (*tree.DeleteNodeResponse, error) {
	if a.isStorePath(in.Node.Path) {
		if !a.AllowPut {
			return nil, errors.Forbidden(VIEWS_LIBRARY_NAME, "Forbidden store")
		}
		dsInfo, er := a.clientsPool.GetDataSourceInfo(a.StoreName)
		if er == nil {
			ctx = WithBranchInfo(ctx, "in", BranchInfo{DSInfo: dsInfo})
			in.Node.SetMeta(common.META_NAMESPACE_DATASOURCE_PATH, path.Base(in.Node.Path))
		}
	}
	return a.next.DeleteNode(ctx, in, opts...)
}

func (a *BinaryStoreHandler) PutObject(ctx context.Context, node *tree.Node, reader io.Reader, requestData *PutRequestData) (int64, error) {
	if a.isStorePath(node.Path) {
		if !a.AllowPut {
			return 0, errors.Forbidden(VIEWS_LIBRARY_NAME, "Forbidden store")
		}
		dsInfo, er := a.clientsPool.GetDataSourceInfo(a.StoreName)
		if er == nil {
			ctx = WithBranchInfo(ctx, "in", BranchInfo{DSInfo: dsInfo})
			node.Uuid = path.Base(node.Path)
			node.SetMeta(common.META_NAMESPACE_DATASOURCE_PATH, path.Base(node.Path))
		}
	}
	return a.next.PutObject(ctx, node, reader, requestData)
}

func (a *BinaryStoreHandler) CopyObject(ctx context.Context, from *tree.Node, to *tree.Node, requestData *CopyRequestData) (int64, error) {
	if a.isStorePath(from.Path) || a.isStorePath(to.Path) {
		return 0, errors.Forbidden(VIEWS_LIBRARY_NAME, "Forbidden store")
	}
	return a.next.CopyObject(ctx, from, to, requestData)
}
