package views

import (
	"github.com/micro/go-micro/errors"
	"github.com/pydio/services/common/proto/idm"
	"github.com/pydio/services/common/proto/tree"
	"golang.org/x/net/context"
	"strings"
)

type UuidNodeHandler struct {
	AbstractBranchFilter
}

func NewUuidNodeHandler() *UuidNodeHandler {
	u := &UuidNodeHandler{}
	u.inputMethod = u.updateInputBranch
	u.outputMethod = u.updateOutputBranch
	return u
}

func (h *UuidNodeHandler) updateInputBranch(ctx context.Context, identifier string, node *tree.Node) (context.Context, error) {

	if info, alreadySet := GetBranchInfo(ctx, identifier); alreadySet && info.DSInfo.Client != nil {
		return ctx, nil
	}

	// We expected a node Uuid composed from workspaceId:Uuid
	var parts []string
	if parts = strings.Split(node.Uuid, ":"); len(parts) != 2 {
		return ctx, errors.InternalServerError(VIEWS_LIBRARY_NAME, "Please provide an Uuid prefixed by a workspace Id")
	}
	var ws *idm.Workspace
	wsId := parts[0]
	uuid := parts[1]

	if workspaces, ok := ctx.Value(ctxUserWorkspacesKey{}).(map[string]*idm.Workspace); ok {
		// Find by slug
		for _, wsV := range workspaces {
			if wsV.Slug == wsId {
				ws = wsV
			}
		}
	}
	if ws == nil {
		return ctx, errors.InternalServerError(VIEWS_LIBRARY_NAME, "Cannot find workspace Id in authorized workspaces")
	}

	// TODO : Query tree to make sure node X is really a child of workspace Root(s) Y

	branchInfo := BranchInfo{}
	branchInfo.Workspace = *ws
	node.Uuid = uuid
	return WithBranchInfo(ctx, identifier, branchInfo), nil

}

func (h *UuidNodeHandler) updateOutputBranch(ctx context.Context, identifier string, node *tree.Node) (context.Context, error) {

	// We have to find back the workspace now
	info, ok := GetBranchInfo(ctx, identifier)
	if !ok {
		return ctx, errors.InternalServerError(VIEWS_LIBRARY_NAME, "Cannot find workspace in branch")
	}
	node.Uuid = info.Slug + ":" + node.Uuid

	return ctx, nil

}
