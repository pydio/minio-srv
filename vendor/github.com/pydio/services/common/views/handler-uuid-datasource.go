package views

import (
	"github.com/micro/go-micro/errors"
	"github.com/pydio/services/common"
	"github.com/pydio/services/common/proto/tree"
	"golang.org/x/net/context"
)

func NewUuidDataSourceHandler() *UuidDataSourceHandler {
	bt := &UuidDataSourceHandler{}
	bt.inputMethod = bt.updateInputBranch
	bt.outputMethod = bt.updateOutputNode
	return bt
}

type UuidDataSourceHandler struct {
	AbstractBranchFilter
}

func (v *UuidDataSourceHandler) updateInputBranch(ctx context.Context, identifier string, node *tree.Node) (context.Context, error) {

	branchInfo, ok := GetBranchInfo(ctx, identifier)
	if !ok {
		return ctx, errors.InternalServerError(VIEWS_LIBRARY_NAME, "Cannot find branch info for node")
	}
	if branchInfo.Client != nil {
		// DS Is already set by a previous middleware, ignore.
		return ctx, nil
	}

	dsName := node.GetStringMeta(common.META_NAMESPACE_DATASOURCE_NAME)
	dsPath := node.GetStringMeta(common.META_NAMESPACE_DATASOURCE_PATH)
	if len(dsPath) == 0 || len(dsName) == 0 {
		// Ignore this step
		return ctx, nil
	}
	dsInfo, e := v.clientsPool.GetDataSourceInfo(dsName)
	if e != nil {
		return ctx, e
	}
	branchInfo.DSInfo = dsInfo
	ctx = WithBranchInfo(ctx, identifier, branchInfo)

	return ctx, nil

}

func (v *UuidDataSourceHandler) updateOutputNode(ctx context.Context, identifier string, node *tree.Node) (context.Context, error) {

	return ctx, nil

}
