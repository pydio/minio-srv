package views

import (
	"io"
	"github.com/pydio/services/common/log"

	"github.com/micro/go-micro/client"
	"github.com/pydio/minio-go"
	"github.com/pydio/services/common"
	"github.com/pydio/services/common/proto/tree"
	"golang.org/x/net/context"
)

func NewStandardRouter(adminView bool, watchRegistry bool) *Router {
	handlers := []Handler{
		NewAuthHandler(adminView),
		&BinaryStoreHandler{
			StoreName: common.PYDIO_THUMBSTORE_NAMESPACE, // Direct access to dedicated Bucket for thumbnails
		},
		&BinaryStoreHandler{
			StoreName:     common.PYDIO_DOCSTORE_BINARIES_NAMESPACE, // Direct access to dedicated Bucket for pydio binaries
			AllowPut:      true,
			AllowAnonRead: true,
		},
		NewPathWorkspaceHandler(),
		NewPathMultipleRootsHandler(),
		NewPathDataSourceHandler(),
		&ArchiveHandler{},    // Catch "GET" request on folder.zip and create archive on-demand
		&EncryptionHandler{}, // Handler retrieve encryption materials from encryption service
		&PutHandler{},        // Handler adding a node precreation on PUT file request
		&VersionHandler{},
		&Executor{},
	}
	pool := NewClientsPool(watchRegistry)
	return NewRouter(pool, handlers)
}

func NewUuidRouter(adminView bool, watchRegistry bool) *Router {
	handlers := []Handler{
		NewAuthHandler(adminView),
		NewUuidNodeHandler(),
		NewUuidRootsHandler(),
		NewUuidDataSourceHandler(),
		&EncryptionHandler{}, // Handler retrieve encryption materials from encryption service
		&PutHandler{},        // Handler adding a node precreation on PUT file request
		&VersionHandler{},
		&Executor{},
	}
	pool := NewClientsPool(watchRegistry)
	return NewRouter(pool, handlers)
}

func NewRouter(pool *ClientsPool, handlers []Handler) *Router {
	r := &Router{
		handlers: handlers,
		pool:     pool,
	}
	r.initHandlers()
	return r
}

type Router struct {
	handlers []Handler
	pool     *ClientsPool
}

func (v *Router) initHandlers() {
	for i, h := range v.handlers {
		if i < len(v.handlers)-1 {
			next := v.handlers[i+1]
			h.SetNextHandler(next)
		}
		h.SetClientsPool(v.pool)
	}
}

func (v *Router) WrapCallback(provider NodesCallback) error{
	return v.ExecuteWrapped(nil, nil, provider)
}

func (v *Router) ExecuteWrapped(inputFilter NodeFilter, outputFilter NodeFilter, provider NodesCallback) error {
	outputFilter = func(ctx context.Context, inputNode *tree.Node, identifier string) (context.Context, error) {
		return ctx, nil
	}
	inputFilter = func(ctx context.Context, inputNode *tree.Node, identifier string) (context.Context, error) {
		return WithTreeClients(ctx, v.pool), nil
	}
	return v.handlers[0].ExecuteWrapped(inputFilter, outputFilter, provider)
}

func (v *Router) ReadNode(ctx context.Context, in *tree.ReadNodeRequest, opts ...client.CallOption) (*tree.ReadNodeResponse, error) {
	h := v.handlers[0]
	ctx = WithTreeClients(ctx, v.pool)
	return h.ReadNode(ctx, in, opts...)
}

func (v *Router) ListNodes(ctx context.Context, in *tree.ListNodesRequest, opts ...client.CallOption) (tree.NodeProvider_ListNodesClient, error) {
	h := v.handlers[0]
	ctx = WithTreeClients(ctx, v.pool)
	log.Logger(ctx).Debug("Listing nodes in router")
	return h.ListNodes(ctx, in, opts...)
}

func (v *Router) CreateNode(ctx context.Context, in *tree.CreateNodeRequest, opts ...client.CallOption) (*tree.CreateNodeResponse, error) {
	h := v.handlers[0]
	ctx = WithTreeClients(ctx, v.pool)
	return h.CreateNode(ctx, in, opts...)
}

func (v *Router) UpdateNode(ctx context.Context, in *tree.UpdateNodeRequest, opts ...client.CallOption) (*tree.UpdateNodeResponse, error) {
	h := v.handlers[0]
	ctx = WithTreeClients(ctx, v.pool)
	return h.UpdateNode(ctx, in, opts...)
}

func (v *Router) DeleteNode(ctx context.Context, in *tree.DeleteNodeRequest, opts ...client.CallOption) (*tree.DeleteNodeResponse, error) {
	h := v.handlers[0]
	ctx = WithTreeClients(ctx, v.pool)
	return h.DeleteNode(ctx, in, opts...)
}

func (v *Router) GetObject(ctx context.Context, node *tree.Node, requestData *GetRequestData) (io.ReadCloser, error) {
	h := v.handlers[0]
	ctx = WithTreeClients(ctx, v.pool)
	return h.GetObject(ctx, node, requestData)
}

func (v *Router) PutObject(ctx context.Context, node *tree.Node, reader io.Reader, requestData *PutRequestData) (int64, error) {
	h := v.handlers[0]
	ctx = WithTreeClients(ctx, v.pool)
	return h.PutObject(ctx, node, reader, requestData)
}

func (v *Router) CopyObject(ctx context.Context, from *tree.Node, to *tree.Node, requestData *CopyRequestData) (int64, error) {
	h := v.handlers[0]
	ctx = WithTreeClients(ctx, v.pool)
	return h.CopyObject(ctx, from, to, requestData)
}

func (v *Router) MultipartCreate(ctx context.Context, target *tree.Node, requestData *MultipartRequestData) (string, error) {
	ctx = WithTreeClients(ctx, v.pool)
	return v.handlers[0].MultipartCreate(ctx, target, requestData)
}

func (v *Router) MultipartList(ctx context.Context, prefix string, requestData *MultipartRequestData) (minio.ListMultipartUploadsResult, error) {
	ctx = WithTreeClients(ctx, v.pool)
	return v.handlers[0].MultipartList(ctx, prefix, requestData)
}

func (v *Router) MultipartAbort(ctx context.Context, target *tree.Node, uploadID string, requestData *MultipartRequestData) error {
	ctx = WithTreeClients(ctx, v.pool)
	return v.handlers[0].MultipartAbort(ctx, target, uploadID, requestData)
}

func (v *Router) MultipartComplete(ctx context.Context, target *tree.Node, uploadID string, uploadedParts []minio.CompletePart) (minio.ObjectInfo, error) {
	ctx = WithTreeClients(ctx, v.pool)
	return v.handlers[0].MultipartComplete(ctx, target, uploadID, uploadedParts)
}

func (v *Router) MultipartListObjectParts(ctx context.Context, target *tree.Node, uploadID string, partNumberMarker int, maxParts int) (minio.ListObjectPartsResult, error) {
	ctx = WithTreeClients(ctx, v.pool)
	return v.handlers[0].MultipartListObjectParts(ctx, target, uploadID, partNumberMarker, maxParts)
}

// To respect Handler interface
func (v *Router) SetNextHandler(h Handler){}
func (v *Router) SetClientsPool(p *ClientsPool){}

// Specific to Router
func (v *Router) GetClientsPool() *ClientsPool {
	return v.pool
}