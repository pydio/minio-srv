package views

import (
	"github.com/pydio/services/common/log"
	"path"
	"strings"

	"github.com/micro/go-micro/errors"
	"github.com/pydio/services/common"
	"github.com/pydio/services/common/proto/tree"
	"go.uber.org/zap"
	"golang.org/x/net/context"
)

func NewPathDataSourceHandler() *PathDataSourceHandler {
	bt := &PathDataSourceHandler{}
	bt.inputMethod = bt.updateInputBranch
	bt.outputMethod = bt.updateOutputNode
	return bt
}

type PathDataSourceHandler struct {
	AbstractBranchFilter
}

func (v *PathDataSourceHandler) updateInputBranch(ctx context.Context, identifier string, node *tree.Node) (context.Context, error) {

	branchInfo, ok := GetBranchInfo(ctx, identifier)
	if !ok {
		return ctx, errors.InternalServerError(VIEWS_LIBRARY_NAME, "Cannot find branch info for node")
	}
	if branchInfo.Client != nil {
		// DS Is already set by a previous middleware, ignore.
		return ctx, nil
	}
	if branchInfo.Workspace.UUID == "ROOT" && len(strings.Trim(node.Path, "/")) > 0 {
		// Get Data Source from first segment, leave tree path unchanged
		parts := strings.Split(strings.Trim(node.Path, "/"), "/")
		dsName := parts[0]
		dsInfo, e := v.clientsPool.GetDataSourceInfo(dsName)
		if e != nil {
			return ctx, e
		}
		if len(parts) > 1 {
			dsPath := strings.Join(parts[1:], "/")
			node.SetMeta(common.META_NAMESPACE_DATASOURCE_PATH, dsPath)
		}
		//log.Logger(ctx).Debug("Setting DSInfo", zap.String("bucket", dsInfo.Bucket))
		branchInfo.DSInfo = dsInfo
		ctx = WithBranchInfo(ctx, identifier, branchInfo)

	} else if branchInfo.Root != nil {

		wsRoot := branchInfo.Root
		originalPath := node.Path
		dsName := wsRoot.GetStringMeta(common.META_NAMESPACE_DATASOURCE_NAME)
		dsPath := wsRoot.GetStringMeta(common.META_NAMESPACE_DATASOURCE_PATH)

		node.Path = path.Join(wsRoot.Path, originalPath)
		log.Logger(ctx).Info("Real Node Path", zap.String("path", node.Path))
		node.SetMeta(common.META_NAMESPACE_DATASOURCE_PATH, path.Join(dsPath, originalPath))
		dsInfo, err := v.clientsPool.GetDataSourceInfo(dsName)
		if err != nil {
			return nil, err
		}
		branchInfo.DSInfo = dsInfo
		ctx = WithBranchInfo(ctx, identifier, branchInfo)

	} else {

		return ctx, errors.InternalServerError(VIEWS_LIBRARY_NAME, "Missing Root in context")

	}

	return ctx, nil

}

func (v *PathDataSourceHandler) updateOutputNode(ctx context.Context, identifier string, node *tree.Node) (context.Context, error) {

	branchInfo, _ := GetBranchInfo(ctx, identifier)
	if branchInfo.Workspace.UUID == "ROOT" {
		// Nothing to do
		return ctx, nil
	}
	if branchInfo.Root == nil {
		return ctx, errors.InternalServerError(VIEWS_LIBRARY_NAME, "No Root defined, this is not normal")
	}
	// Trim root path, and append workspace ID
	node.Path = strings.Trim(strings.TrimPrefix(node.Path, branchInfo.Root.Path), "/")

	return ctx, nil

}
