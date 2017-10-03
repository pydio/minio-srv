package views

import (
	"io"

	"github.com/minio/minio-go/pkg/encrypt"
	"github.com/pydio/minio-go"
	"github.com/pydio/services/common/proto/idm"
	"github.com/pydio/services/common/proto/tree"
	"golang.org/x/net/context"
)

const (
	VIEWS_LIBRARY_NAME = "pydio.lib.views"
)

// These keys may be enriched in Context depending on the middleware
type (
	ctxUserIdKey struct{}

	ctxUserWorkspacesKey struct{}

	ctxAdminContextKey struct{}

	// NodeProviderClient
	ctxTreeReader struct{}
	// NodeReceiverClient
	ctxTreeWriter struct{}

	ctxBranchInfoKey struct{}

	DSInfo struct {
		Client    *minio.Core
		Bucket    string
		Encrypted bool
	}

	BranchInfo struct {
		Root *tree.Node
		DSInfo
		idm.Workspace
	}

	PutRequestData struct {
		Size               int64
		Md5Sum             []byte
		Sha256Sum          []byte
		Metadata           map[string][]string
		EncryptionMaterial encrypt.Materials
		MultipartUploadID  string
		MultipartPartID    int
	}

	GetRequestData struct {
		StartOffset        int64
		Length             int64
		EncryptionMaterial encrypt.Materials
		VersionId          string
	}

	CopyRequestData struct {
		Metadata               map[string][]string
		srcEncryptionMaterial  encrypt.Materials
		destEncryptionMaterial encrypt.Materials
		srcVersionId           string
	}

	MultipartRequestData struct {
		Metadata map[string]string

		ListKeyMarker      string
		ListUploadIDMarker string
		ListDelimiter      string
		ListMaxUploads     int
	}
)

type NodeFilter func(ctx context.Context, inputNode *tree.Node, identifier string) (context.Context, error)
type NodesCallback func(inputFilter NodeFilter, outputFilter NodeFilter) error

type Handler interface {
	tree.NodeProviderClient
	tree.NodeReceiverClient
	GetObject(ctx context.Context, node *tree.Node, requestData *GetRequestData) (io.ReadCloser, error)
	PutObject(ctx context.Context, node *tree.Node, reader io.Reader, requestData *PutRequestData) (int64, error)
	CopyObject(ctx context.Context, from *tree.Node, to *tree.Node, requestData *CopyRequestData) (int64, error)

	MultipartCreate(ctx context.Context, target *tree.Node, requestData *MultipartRequestData) (string, error)
	MultipartList(ctx context.Context, prefix string, requestData *MultipartRequestData) (minio.ListMultipartUploadsResult, error)
	MultipartAbort(ctx context.Context, target *tree.Node, uploadID string, requestData *MultipartRequestData) error
	MultipartComplete(ctx context.Context, target *tree.Node, uploadID string, uploadedParts []minio.CompletePart) (minio.ObjectInfo, error)
	MultipartListObjectParts(ctx context.Context, target *tree.Node, uploadID string, partNumberMarker int, maxParts int) (minio.ListObjectPartsResult, error)

	ExecuteWrapped(inputFilter NodeFilter, outputFilter NodeFilter, provider NodesCallback) error

	SetNextHandler(h Handler)
	SetClientsPool(p *ClientsPool)
}

func TreeClientsFromContext(ctx context.Context) (tree.NodeProviderClient, tree.NodeReceiverClient) {
	return ctx.Value(ctxTreeReader{}).(tree.NodeProviderClient), ctx.Value(ctxTreeWriter{}).(tree.NodeReceiverClient)
}

func WithTreeClients(ctx context.Context, pool *ClientsPool) context.Context {
	ctx = context.WithValue(ctx, ctxTreeReader{}, pool.TreeClient)
	ctx = context.WithValue(ctx, ctxTreeWriter{}, pool.TreeClientWrite)
	return ctx
}

func WithBranchInfo(ctx context.Context, identifier string, branchInfo BranchInfo) context.Context {
	value := ctx.Value(ctxBranchInfoKey{})
	var data map[string]BranchInfo
	if value != nil {
		data = value.(map[string]BranchInfo)
	} else {
		data = make(map[string]BranchInfo)
	}
	data[identifier] = branchInfo
	return context.WithValue(ctx, ctxBranchInfoKey{}, data)
}

func GetBranchInfo(ctx context.Context, identifier string) (BranchInfo, bool) {
	value := ctx.Value(ctxBranchInfoKey{})
	if value != nil {
		data := value.(map[string]BranchInfo)
		if info, ok := data[identifier]; ok {
			return info, true
		}
	}
	return BranchInfo{}, false
}

func UserWorkspacesFromContext(ctx context.Context) map[string]*idm.Workspace {
	return ctx.Value(ctxUserWorkspacesKey{}).(map[string]*idm.Workspace)
}
