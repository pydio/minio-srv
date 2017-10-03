package views

import (
	"github.com/pydio/services/common/log"
	"strings"

	"github.com/micro/go-micro/client"
	"github.com/micro/go-micro/errors"
	"github.com/pydio/services/common/proto/tree"
	"go.uber.org/zap"
	"golang.org/x/net/context"
)

type MultipleRootsHandler struct {
	AbstractBranchFilter
}

func NewPathMultipleRootsHandler() *MultipleRootsHandler {
	m := &MultipleRootsHandler{}
	m.outputMethod = m.updateOutputBranch
	m.inputMethod = m.updateInputBranch
	return m
}

func (m *MultipleRootsHandler) updateInputBranch(ctx context.Context, identifier string, node *tree.Node) (context.Context, error) {

	branch, set := GetBranchInfo(ctx, identifier)
	//log.Logger(ctx).Debug("updateInputBranch", zap.Any("branch", branch), zap.Bool("set", set))
	if !set || branch.UUID == "ROOT" || branch.Client != nil {
		return ctx, nil
	}
	if len(branch.RootNodes) == 1 {
		rootNode, err := m.getRoot(branch.RootNodes[0])
		if err != nil {
			return ctx, err
		}
		branch.Root = rootNode
		ctx = WithBranchInfo(ctx, identifier, branch)
		return ctx, nil
	}

	// There are multiple root nodes: detect /node-uuid/ segment in the path
	parts := strings.Split(strings.Trim(node.Path, "/"), "/")
	if len(parts) > 0 {
		rootId := parts[0]
		log.Logger(ctx).Debug("Searching", zap.String("root", rootId), zap.Any("rootNodes", branch.RootNodes))
		rootKeys, e := m.rootKeysMap(branch.RootNodes)
		if e != nil {
			return ctx, e
		}
		for key, rNode := range rootKeys {
			if key == rootId || rootId == rNode.GetUuid() {
				branch.Root = rNode
				node.Path = strings.Join(parts[1:], "/") // Replace path parts
				ctx = WithBranchInfo(ctx, identifier, branch)
				break
			}
		}
	}
	if branch.Root == nil {
		return ctx, errors.NotFound(VIEWS_LIBRARY_NAME, "Could not find root node")
	}
	return ctx, nil
}

func (m *MultipleRootsHandler) updateOutputBranch(ctx context.Context, identifier string, node *tree.Node) (context.Context, error) {

	branch, set := GetBranchInfo(ctx, identifier)
	if !set || branch.UUID == "ROOT" || len(branch.RootNodes) < 2 {
		return ctx, nil
	}
	if branch.Root == nil {
		return ctx, errors.InternalServerError(VIEWS_LIBRARY_NAME, "No Root defined, this is not normal")
	}
	// Prepend root node Uuid
	node.Path = m.makeRootKey(branch.Root) + "/" + strings.TrimLeft(node.Path, "/")

	return ctx, nil
}

func (m *MultipleRootsHandler) ListNodes(ctx context.Context, in *tree.ListNodesRequest, opts ...client.CallOption) (tree.NodeProvider_ListNodesClient, error) {

	// First try, without modifying ctx & node
	_, err := m.updateInputBranch(ctx, "in", &tree.Node{Path: in.Node.Path})
	if err != nil && errors.Parse(err.Error()).Status == "Not Found" {

		branch, _ := GetBranchInfo(ctx, "in")
		streamer := NewWrappingStreamer()
		nodes, e := m.rootKeysMap(branch.RootNodes)
		if e != nil {
			return streamer, e
		}
		go func() {
			defer streamer.Close()
			for rKey, rNode := range nodes {
				node := &tree.Node{
					Type: tree.NodeType_COLLECTION,
					Uuid: rNode.Uuid,
					Path: rKey,
				}
				node.SetMeta("name", rNode.GetStringMeta("name"))
				streamer.Send(&tree.ListNodesResponse{Node: node})
			}
		}()
		return streamer, nil
	}
	return m.AbstractBranchFilter.ListNodes(ctx, in, opts...)

}

func (m *MultipleRootsHandler) ReadNode(ctx context.Context, in *tree.ReadNodeRequest, opts ...client.CallOption) (*tree.ReadNodeResponse, error) {

	// First try, without modifying ctx & node
	_, err := m.updateInputBranch(ctx, "in", &tree.Node{Path: in.Node.Path})
	if err != nil && errors.Parse(err.Error()).Status == "Not Found" {
		// Return a fake root node
		return &tree.ReadNodeResponse{Success: true, Node: &tree.Node{Path: ""}}, nil
	}
	return m.AbstractBranchFilter.ReadNode(ctx, in, opts...)

}
