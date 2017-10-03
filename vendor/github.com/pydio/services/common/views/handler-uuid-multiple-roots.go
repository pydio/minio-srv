package views

import (
	"github.com/micro/go-micro/errors"
	"github.com/pydio/services/common/proto/tree"
	"golang.org/x/net/context"
	"strings"
)

type UuidRootsHandler struct {
	AbstractBranchFilter
}

func NewUuidRootsHandler() *UuidRootsHandler {
	u := &UuidRootsHandler{}
	u.inputMethod = u.updateInputBranch
	u.outputMethod = u.updateOutputBranch
	return u
}

func (h *UuidRootsHandler) updateInputBranch(ctx context.Context, identifier string, node *tree.Node) (context.Context, error) {

	return ctx, nil

}

func (h *UuidRootsHandler) updateOutputBranch(ctx context.Context, identifier string, node *tree.Node) (context.Context, error) {

	// Rebuild the path now
	branch, set := GetBranchInfo(ctx, identifier)
	if !set || branch.UUID == "ROOT" {
		return ctx, nil
	}
	if len(branch.RootNodes) == 0 {
		return ctx, errors.InternalServerError(VIEWS_LIBRARY_NAME, "Cannot find roots for workspace")
	}

	multiRootKey := ""
	var detectedRoot *tree.Node
	if len(branch.RootNodes) > 1 {
		// Root is not set, find it now
		wsRoots, err := h.rootKeysMap(branch.RootNodes)
		if err != nil {
			return ctx, err
		}
		for _, rNode := range wsRoots {
			if strings.HasPrefix(node.Path, rNode.Path) {
				detectedRoot = rNode
				break
			}
		}
		if detectedRoot == nil {
			return ctx, errors.InternalServerError(VIEWS_LIBRARY_NAME, "Cannot find root node in workspace, this should not happen")
		}
		multiRootKey = h.makeRootKey(detectedRoot) + "/"
	} else {
		var err error
		detectedRoot, err = h.getRoot(branch.RootNodes[0])
		if err != nil {
			return ctx, err
		}
	}

	node.Path = branch.Slug + "/" + multiRootKey + strings.TrimLeft(strings.TrimPrefix(node.Path, detectedRoot.Path), "/")

	return ctx, nil

}
