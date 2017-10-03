package views

import (
	"github.com/micro/go-micro/client"
	"github.com/pydio/minio-go"
	"github.com/pydio/services/common/proto/tree"
	"golang.org/x/net/context"
	"io"
)

type ContextWrapper func(ctx context.Context) (context.Context, error)

// Abstract Handler implementation simply forwards
// calls to the next handler
type AbstractHandler struct {
	next        Handler
	clientsPool *ClientsPool
	CtxWrapper  ContextWrapper
}

func (a *AbstractHandler) wrapContext(ctx context.Context) (context.Context, error) {
	if a.CtxWrapper != nil {
		return a.CtxWrapper(ctx)
	} else {
		return ctx, nil
	}
}

func (a *AbstractHandler) SetNextHandler(h Handler) {
	a.next = h
}

func (a *AbstractHandler) SetClientsPool(p *ClientsPool) {
	a.clientsPool = p
}

func (a *AbstractHandler) ExecuteWrapped(inputFilter NodeFilter, outputFilter NodeFilter, provider NodesCallback) error {
	wrappedIn := func(ctx context.Context, inputNode *tree.Node, identifier string) (context.Context, error) {
		ctx, err := inputFilter(ctx, inputNode, identifier)
		if err != nil {
			return ctx, err
		}
		ctx, err = a.wrapContext(ctx)
		if err != nil {
			return ctx, err
		}
		return ctx, nil
	}
	return a.next.ExecuteWrapped(wrappedIn, outputFilter, provider)
}

func (a *AbstractHandler) ReadNode(ctx context.Context, in *tree.ReadNodeRequest, opts ...client.CallOption) (*tree.ReadNodeResponse, error) {
	ctx, err := a.wrapContext(ctx)
	if err != nil {
		return nil, err
	}
	return a.next.ReadNode(ctx, in, opts...)
}

func (a *AbstractHandler) ListNodes(ctx context.Context, in *tree.ListNodesRequest, opts ...client.CallOption) (tree.NodeProvider_ListNodesClient, error) {
	ctx, err := a.wrapContext(ctx)
	if err != nil {
		return nil, err
	}
	return a.next.ListNodes(ctx, in, opts...)
}

func (a *AbstractHandler) CreateNode(ctx context.Context, in *tree.CreateNodeRequest, opts ...client.CallOption) (*tree.CreateNodeResponse, error) {
	ctx, err := a.wrapContext(ctx)
	if err != nil {
		return nil, err
	}
	return a.next.CreateNode(ctx, in, opts...)
}

func (a *AbstractHandler) UpdateNode(ctx context.Context, in *tree.UpdateNodeRequest, opts ...client.CallOption) (*tree.UpdateNodeResponse, error) {
	ctx, err := a.wrapContext(ctx)
	if err != nil {
		return nil, err
	}
	return a.next.UpdateNode(ctx, in, opts...)
}

func (a *AbstractHandler) DeleteNode(ctx context.Context, in *tree.DeleteNodeRequest, opts ...client.CallOption) (*tree.DeleteNodeResponse, error) {
	ctx, err := a.wrapContext(ctx)
	if err != nil {
		return nil, err
	}
	return a.next.DeleteNode(ctx, in, opts...)
}

func (a *AbstractHandler) GetObject(ctx context.Context, node *tree.Node, requestData *GetRequestData) (io.ReadCloser, error) {
	ctx, err := a.wrapContext(ctx)
	if err != nil {
		return nil, err
	}
	return a.next.GetObject(ctx, node, requestData)
}

func (a *AbstractHandler) PutObject(ctx context.Context, node *tree.Node, reader io.Reader, requestData *PutRequestData) (int64, error) {
	ctx, err := a.wrapContext(ctx)
	if err != nil {
		return 0, err
	}
	return a.next.PutObject(ctx, node, reader, requestData)
}

func (a *AbstractHandler) CopyObject(ctx context.Context, from *tree.Node, to *tree.Node, requestData *CopyRequestData) (int64, error) {
	ctx, err := a.wrapContext(ctx)
	if err != nil {
		return 0, err
	}
	return a.next.CopyObject(ctx, from, to, requestData)
}

func (a *AbstractHandler) MultipartCreate(ctx context.Context, target *tree.Node, requestData *MultipartRequestData) (string, error) {
	ctx, err := a.wrapContext(ctx)
	if err != nil {
		return "", err
	}
	return a.next.MultipartCreate(ctx, target, requestData)
}

func (a *AbstractHandler) MultipartList(ctx context.Context, prefix string, requestData *MultipartRequestData) (minio.ListMultipartUploadsResult, error) {
	ctx, err := a.wrapContext(ctx)
	if err != nil {
		return minio.ListMultipartUploadsResult{}, err
	}
	return a.next.MultipartList(ctx, prefix, requestData)
}

func (a *AbstractHandler) MultipartAbort(ctx context.Context, target *tree.Node, uploadID string, requestData *MultipartRequestData) error {
	ctx, err := a.wrapContext(ctx)
	if err != nil {
		return err
	}
	return a.next.MultipartAbort(ctx, target, uploadID, requestData)
}

func (a *AbstractHandler) MultipartComplete(ctx context.Context, target *tree.Node, uploadID string, uploadedParts []minio.CompletePart) (minio.ObjectInfo, error) {
	ctx, err := a.wrapContext(ctx)
	if err != nil {
		return minio.ObjectInfo{}, err
	}
	return a.next.MultipartComplete(ctx, target, uploadID, uploadedParts)
}

func (a *AbstractHandler) MultipartListObjectParts(ctx context.Context, target *tree.Node, uploadID string, partNumberMarker int, maxParts int) (minio.ListObjectPartsResult, error) {
	ctx, err := a.wrapContext(ctx)
	if err != nil {
		return minio.ListObjectPartsResult{}, err
	}
	return a.next.MultipartListObjectParts(ctx, target, uploadID, partNumberMarker, maxParts)
}
