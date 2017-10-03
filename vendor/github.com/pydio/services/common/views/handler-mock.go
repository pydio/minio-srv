package views

import (
	"errors"
	"github.com/micro/go-micro/client"
	"github.com/pydio/minio-go"
	"github.com/pydio/services/common/proto/tree"
	"golang.org/x/net/context"
	"io"
	"strings"
	"github.com/pydio/services/common/log"
)

func NewHandlerMock() *HandlerMock {
	return &HandlerMock{
		Nodes: make(map[string]*tree.Node),
	}
}

type HandlerMock struct {
	Nodes   map[string]*tree.Node
	Context context.Context
}

type MockReadCloser struct {
	io.Reader
}

func (r MockReadCloser) Close() error {
	return nil
}

func (h *HandlerMock) SetNextHandler(handler Handler) {}

func (h *HandlerMock) SetClientsPool(p *ClientsPool) {}

func (h *HandlerMock) ExecuteWrapped(inputFilter NodeFilter, outputFilter NodeFilter, provider NodesCallback) error {
	return provider(inputFilter, outputFilter)
}

func (h *HandlerMock) ReadNode(ctx context.Context, in *tree.ReadNodeRequest, opts ...client.CallOption) (*tree.ReadNodeResponse, error) {
	h.Nodes["in"] = in.Node
	h.Context = ctx
	if n, ok := h.Nodes[in.Node.Path]; ok {
		return &tree.ReadNodeResponse{Node: n}, nil
	}
	return nil, errors.New("Not Found")
}

func (h *HandlerMock) ListNodes(ctx context.Context, in *tree.ListNodesRequest, opts ...client.CallOption) (tree.NodeProvider_ListNodesClient, error) {
	h.Nodes["in"] = in.Node
	h.Context = ctx
	streamer := NewWrappingStreamer()
	go func() {
		defer streamer.Close()
		for _, n := range h.Nodes {
			if strings.HasPrefix(n.Path, in.Node.Path+"/") {
				streamer.Send(&tree.ListNodesResponse{Node: n})
			}
		}
	}()
	return streamer, nil
}

func (h *HandlerMock) CreateNode(ctx context.Context, in *tree.CreateNodeRequest, opts ...client.CallOption) (*tree.CreateNodeResponse, error) {
	log.Logger(ctx).Info("[MOCK] Create Node " + in.Node.Path)
	h.Nodes["in"] = in.Node
	h.Context = ctx
	return nil, nil
}

func (h *HandlerMock) UpdateNode(ctx context.Context, in *tree.UpdateNodeRequest, opts ...client.CallOption) (*tree.UpdateNodeResponse, error) {
	log.Logger(ctx).Info("[MOCK] Update Node " + in.From.Path + " to " + in.To.Path)
	h.Nodes["from"] = in.From
	h.Nodes["to"] = in.To
	h.Context = ctx
	return nil, nil
}

func (h *HandlerMock) DeleteNode(ctx context.Context, in *tree.DeleteNodeRequest, opts ...client.CallOption) (*tree.DeleteNodeResponse, error) {
	log.Logger(ctx).Info("[MOCK] Delete Node" + in.Node.Path)
	h.Nodes["in"] = in.Node
	h.Context = ctx
	return nil, nil
}

func (h *HandlerMock) GetObject(ctx context.Context, node *tree.Node, requestData *GetRequestData) (io.ReadCloser, error) {
	h.Nodes["in"] = node
	h.Context = ctx
	if n, ok := h.Nodes[node.Path]; ok {
		// Fake node content : node path + hello world
		closer := MockReadCloser{}
		closer.Reader = strings.NewReader(n.Path + "hello world")
		return closer, nil
	}
	return nil, errors.New("Not Found")
}

func (h *HandlerMock) PutObject(ctx context.Context, node *tree.Node, reader io.Reader, requestData *PutRequestData) (int64, error) {
	log.Logger(ctx).Info("[MOCK] PutObject" + node.Path)
	h.Nodes["in"] = node
	h.Context = ctx
	return 0, nil
}

func (h *HandlerMock) CopyObject(ctx context.Context, from *tree.Node, to *tree.Node, requestData *CopyRequestData) (int64, error) {
	log.Logger(ctx).Info("[MOCK] CopyObject " + from.Path + " to " + to.Path)
	h.Nodes["from"] = from
	h.Nodes["to"] = to
	h.Context = ctx
	return 0, nil
}

func (h *HandlerMock) MultipartCreate(ctx context.Context, target *tree.Node, requestData *MultipartRequestData) (string, error) {
	h.Nodes["in"] = target
	h.Context = ctx
	return "", nil
}

func (h *HandlerMock) MultipartList(ctx context.Context, prefix string, requestData *MultipartRequestData) (minio.ListMultipartUploadsResult, error) {
	h.Context = ctx
	return minio.ListMultipartUploadsResult{}, nil
}

func (h *HandlerMock) MultipartAbort(ctx context.Context, target *tree.Node, uploadID string, requestData *MultipartRequestData) error {
	h.Context = ctx
	h.Nodes["in"] = target
	return nil
}

func (h *HandlerMock) MultipartComplete(ctx context.Context, target *tree.Node, uploadID string, uploadedParts []minio.CompletePart) (minio.ObjectInfo, error) {
	h.Nodes["in"] = target
	h.Context = ctx
	return minio.ObjectInfo{}, nil
}

func (h *HandlerMock) MultipartListObjectParts(ctx context.Context, target *tree.Node, uploadID string, partNumberMarker int, maxParts int) (minio.ListObjectPartsResult, error) {
	h.Nodes["in"] = target
	h.Context = ctx
	return minio.ListObjectPartsResult{}, nil
}
