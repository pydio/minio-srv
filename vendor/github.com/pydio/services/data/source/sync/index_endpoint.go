package sync

import (
	"errors"
	"strings"

	"go.uber.org/zap"

	commonsync "github.com/pydio/poc/sync/common"
	"github.com/pydio/services/common/log"
	"github.com/pydio/services/common/proto/tree"
	"golang.org/x/net/context"
)

type IndexEndpoint struct {
	readerClient tree.NodeProviderClient
	writerClient tree.NodeReceiverClient
}

func (i *IndexEndpoint) GetEndpointInfo() commonsync.EndpointInfo {

	return commonsync.EndpointInfo{
		RequiresFoldersRescan: false,
		RequiresNormalization: false,
	}

}

func (i *IndexEndpoint) Walk(walknFc commonsync.WalkNodesFunc, pathes ...string) (err error) {

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	responseClient, e := i.readerClient.ListNodes(ctx, &tree.ListNodesRequest{
		Node: &tree.Node{
			Path: "",
		},
		Recursive: true,
	})
	if e != nil {
		return e
	}
	defer responseClient.Close()
	for {
		response, rErr := responseClient.Recv()
		if rErr != nil {
			walknFc("", nil, rErr)
		}
		if response == nil {
			break
		}
		response.Node.Path = strings.TrimLeft(response.Node.Path, "/")
		walknFc(response.Node.Path, response.Node, nil)
	}
	return nil
}

func (i *IndexEndpoint) Watch(recursivePath string) (*commonsync.WatchObject, error) {
	return nil, errors.New("Watch Not Implemented")
}

func (i *IndexEndpoint) LoadNode(ctx context.Context, path string, leaf ...bool) (node *tree.Node, err error) {

	log.Logger(ctx).Info("LoadNode", zap.String("path", path))

	resp, e := i.readerClient.ReadNode(ctx, &tree.ReadNodeRequest{
		Node: &tree.Node{
			Path: path,
		},
	})
	if e != nil {
		return nil, e
	}
	return resp.Node, nil

}

func (i *IndexEndpoint) CreateNode(ctx context.Context, node *tree.Node, updateIfExists bool) (err error) {


	_, err = i.writerClient.CreateNode(ctx, &tree.CreateNodeRequest{
		Node:           node,
		UpdateIfExists: updateIfExists,
	})

	log.Logger(ctx).Info("CreateNode", zap.Any("node", node), zap.Error(err))

	return err
}

func (i *IndexEndpoint) UpdateNode(ctx context.Context, node *tree.Node) error {
	return i.CreateNode(ctx, node, true)
}

func (i *IndexEndpoint) DeleteNode(ctx context.Context, path string) (err error) {

	log.Logger(ctx).Info("DeleteNode", zap.String("path", path))

	_, err = i.writerClient.DeleteNode(ctx, &tree.DeleteNodeRequest{
		Node: &tree.Node{
			Path: path,
		},
	})
	return err
}

func (i *IndexEndpoint) MoveNode(ctx context.Context, oldPath string, newPath string) (err error) {

	log.Logger(ctx).Info("MoveNode", zap.String("oldPath", oldPath), zap.String("newPath", newPath))

	_, err = i.writerClient.UpdateNode(ctx, &tree.UpdateNodeRequest{
		From: &tree.Node{
			Path: oldPath,
		},
		To: &tree.Node{
			Path: newPath,
		},
	})
	return err
}

func NewIndexEndpoint(dsName string, reader tree.NodeProviderClient, writer tree.NodeReceiverClient) *IndexEndpoint {
	return &IndexEndpoint{
		readerClient: reader,
		writerClient: writer,
	}
}
