package views

import (
	"github.com/micro/go-micro/client"
	"github.com/micro/go-micro/errors"
	"github.com/pydio/services/common/proto/idm"
	"github.com/pydio/services/common/proto/tree"
	"golang.org/x/net/context"
	"strings"
)

type PathWorkspaceHandler struct {
	AbstractBranchFilter
}

func NewPathWorkspaceHandler() *PathWorkspaceHandler {
	u := &PathWorkspaceHandler{}
	u.inputMethod = u.updateBranchInfo
	u.outputMethod = u.updateOutputBranch
	return u
}

func (a *PathWorkspaceHandler) extractWs(ctx context.Context, node *tree.Node) (*idm.Workspace, bool) {

	// Admin context, fake workspace with root ROOT
	if admin, a := ctx.Value(ctxAdminContextKey{}).(bool); admin && a {
		ws := &idm.Workspace{}
		ws.UUID = "ROOT"
		ws.RootNodes = []string{"ROOT"}
		ws.Slug = "ROOT"
		return ws, true
	}

	// User context, folder path must start with /wsId/ or we are listing the root.
	if workspaces, ok := ctx.Value(ctxUserWorkspacesKey{}).(map[string]*idm.Workspace); ok {
		parts := strings.Split(strings.Trim(node.Path, "/"), "/")
		if len(parts) > 0 {
			// Find by slug
			for _, ws := range workspaces {
				if ws.Slug == parts[0] {
					node.Path = strings.Join(parts[1:], "/")
					return ws, true
				}
			}
		}
	}

	return nil, false
}

func (a *PathWorkspaceHandler) updateBranchInfo(ctx context.Context, identifier string, node *tree.Node) (context.Context, error) {
	if info, alreadySet := GetBranchInfo(ctx, identifier); alreadySet && info.DSInfo.Client != nil {
		return ctx, nil
	}
	branchInfo := BranchInfo{}
	if ws, ok := a.extractWs(ctx, node); ok {
		branchInfo.Workspace = *ws
		return WithBranchInfo(ctx, identifier, branchInfo), nil
	}
	return ctx, errors.NotFound(VIEWS_LIBRARY_NAME, "Workspace not found in Path")
}

func (a *PathWorkspaceHandler) updateOutputBranch(ctx context.Context, identifier string, node *tree.Node) (context.Context, error) {
	// Prepend Slug to path
	if info, set := GetBranchInfo(ctx, identifier); set && info.UUID != "ROOT" {

		node.Path = info.Slug + "/" + node.Path

	}

	return ctx, nil
}

func (a *PathWorkspaceHandler) ReadNode(ctx context.Context, in *tree.ReadNodeRequest, opts ...client.CallOption) (*tree.ReadNodeResponse, error) {
	_, wsFound := a.updateBranchInfo(ctx, "in", &tree.Node{Path: in.Node.Path})
	if wsFound != nil && errors.Parse(wsFound.Error()).Status == "Not Found" {
		// Return a fake root node
		return &tree.ReadNodeResponse{true, &tree.Node{Path: ""}}, nil
	}
	return a.AbstractBranchFilter.ReadNode(ctx, in, opts...)

}

func (a *PathWorkspaceHandler) ListNodes(ctx context.Context, in *tree.ListNodesRequest, opts ...client.CallOption) (tree.NodeProvider_ListNodesClient, error) {
	_, wsFound := a.updateBranchInfo(ctx, "in", &tree.Node{Path: in.Node.Path})
	if wsFound != nil && errors.Parse(wsFound.Error()).Status == "Not Found" {
		// List user workspaces here
		workspaces, ok := ctx.Value(ctxUserWorkspacesKey{}).(map[string]*idm.Workspace)
		if !ok {
			return nil, errors.InternalServerError(VIEWS_LIBRARY_NAME, "Cannot find user workspaces")
		}
		streamer := NewWrappingStreamer()
		go func() {
			defer streamer.Close()
			for _, ws := range workspaces {
				if len(ws.RootNodes) > 0 {
					node := &tree.Node{
						Type: tree.NodeType_COLLECTION,
						Uuid: ws.RootNodes[0],
						Path: ws.Slug,
					}
					streamer.Send(&tree.ListNodesResponse{Node: node})
				}
			}
		}()
		return streamer, nil
	}
	return a.AbstractBranchFilter.ListNodes(ctx, in, opts...)
}
