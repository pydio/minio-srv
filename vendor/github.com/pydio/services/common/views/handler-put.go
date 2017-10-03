package views

import (
	"bytes"
	"io"
	"github.com/pydio/services/common/log"
	"strings"
	"time"

	"github.com/pydio/services/common/proto/tree"
	"go.uber.org/zap"
	"golang.org/x/net/context"
)

type PutHandler struct {
	AbstractHandler
}

type onCreateErrorFunc func()

// Create a temporary node before calling a Put request. If it is an update, should send back the already existing node
// Returns the node, a flag to tell wether it is created or not, and eventually an error
// The Put event will afterward update the index
func (m *PutHandler) GetOrCreatePutNode(ctx context.Context, nodePath string, size int64) (*tree.Node, error, onCreateErrorFunc) {

	treeReader, treeWriter := TreeClientsFromContext(ctx)
	treePath := strings.TrimLeft(nodePath, "/")
	existingResp, err := treeReader.ReadNode(ctx, &tree.ReadNodeRequest{
		Node: &tree.Node{
			Path: treePath,
		},
	})
	if err == nil && existingResp.Node != nil {
		return existingResp.Node, nil, nil
	}
	tmpNode := &tree.Node{
		Path:  		treePath,
		MTime: 		time.Now().Unix(),
		Size:  		size,
		Type:  		tree.NodeType_LEAF,
		Etag: 		"temporary",
	}
	createResp, er := treeWriter.CreateNode(ctx, &tree.CreateNodeRequest{Node: tmpNode})
	if er != nil {
		return nil, er, nil
	}
	errorFunc := func() {
		treeWriter.DeleteNode(ctx, &tree.DeleteNodeRequest{Node: createResp.Node})
	}
	return createResp.Node, nil, errorFunc

}

func (m *PutHandler) PutObject(ctx context.Context, node *tree.Node, reader io.Reader, requestData *PutRequestData) (int64, error) {

	if strings.HasSuffix(node.Path, ".__pydio") {
		return m.next.PutObject(ctx, node, reader, requestData)
	}
	if requestData.Metadata == nil {
		requestData.Metadata = make(map[string][]string)
	}

	if node.Uuid != "" {

		log.Logger(ctx).Debug("PUT: Appending node Uuid to request metadata: " + node.Uuid)
		requestData.Metadata["X-Amz-Meta-Pydio-Node-Uuid"] = []string{node.Uuid}
		return m.next.PutObject(ctx, node, reader, requestData)

	} else {
		// PreCreate a node in the tree.
		newNode, nodeErr, onErrorFunc := m.GetOrCreatePutNode(ctx, node.Path, requestData.Size)
		log.Logger(ctx).Info("PreLoad or PreCreate Node in tree", zap.String("path", node.Path), zap.Any("node", newNode), zap.Error(nodeErr))
		if nodeErr != nil {
			return 0, nodeErr
		}
		if !newNode.IsLeaf() {
			// This was a .__pydio and the folder already exists, replace the content
			// with the actual folder Uuid to avoid replacing it
			// We should never pass there???
			reader = bytes.NewBufferString(newNode.Uuid)
		}

		if requestData.Metadata == nil {
			requestData.Metadata = make(map[string][]string)
		}
		requestData.Metadata["X-Amz-Meta-Pydio-Node-Uuid"] = []string{newNode.Uuid}
		size, err := m.next.PutObject(ctx, node, reader, requestData)
		if err != nil && onErrorFunc != nil {
			log.Logger(ctx).Info("Return of PutObject", zap.String("path", node.Path), zap.Int64("size", size), zap.Error(err))
			onErrorFunc()
		}
		return size, err

	}

}
