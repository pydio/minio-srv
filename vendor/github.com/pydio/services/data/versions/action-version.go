package versions

import (
	"github.com/micro/go-micro/client"
	"github.com/pydio/services/common/proto/jobs"
	"github.com/pydio/services/common/views"
	"golang.org/x/net/context"
	"github.com/pydio/services/common/log"
	"github.com/pydio/services/common"
	"github.com/pydio/services/common/proto/tree"
	"go.uber.org/zap"
)

type VersionAction struct {
	Handler       views.Handler
	Pool 		  *views.ClientsPool
	VersionClient tree.NodeVersionerClient
}

var (
	versionActionName = "actions.versioning.create"
)

// Unique identifier
func (c *VersionAction) GetName() string {
	return versionActionName
}

// Pass parameters
func (c *VersionAction) Init(job *jobs.Job, cl client.Client, action *jobs.Action) error {

	router := views.NewStandardRouter(true, false)
	c.Pool = router.GetClientsPool()
	c.Handler = router
	c.VersionClient = tree.NewNodeVersionerClient(common.SERVICE_VERSIONS, cl)
	return nil
}

// Run the actual action code
func (c *VersionAction) Run(ctx context.Context, input jobs.ActionMessage) (jobs.ActionMessage, error) {

	if len(input.Nodes) == 0 {
		return input.WithIgnore(), nil // Ignore
	}
	node := input.Nodes[0]

	if node.Etag == "temporary" {
		return input.WithIgnore(), nil // Ignore
	}

	log.Logger(ctx).Debug("[VERSIONING] Run action now")

	// TODO: find clients from pool so that they are considered the same by the CopyObject request

	dsInfo, e := c.Pool.GetDataSourceInfo(common.PYDIO_VERSIONS_NAMESPACE)
	if e != nil {
		return input.WithError(e), e
	}
	// Prepare ctx with info about the target branch
	ctx = views.WithBranchInfo(ctx, "to", views.BranchInfo{DSInfo:dsInfo})

	resp, err := c.VersionClient.CreateVersion(ctx, &tree.CreateVersionRequest{Node:node})
	if err != nil {
		return input.WithError(err), err
	}
	if (resp.Version == nil || resp.Version == &tree.ChangeLog{}) {
		// No version returned, means content did not change, do not update
		log.Logger(ctx).Info("[VERSIONING] Content did not change, do not update")
		return input.WithIgnore(), nil
	}

	targetNode := &tree.Node{
		Path:node.Uuid + "__" + resp.Version.Uuid,
	}
	targetNode.SetMeta(common.META_NAMESPACE_DATASOURCE_PATH, targetNode.Path)

	written, err := c.Handler.CopyObject(ctx, node, targetNode, &views.CopyRequestData{})

	if err == nil && written > 0 {
		_, err2 := c.VersionClient.StoreVersion(ctx, &tree.StoreVersionRequest{Node: node, Version: resp.Version})
		if err2 != nil{
			return input.WithError(err2), err2
		}
	}

	log.Logger(ctx).Info("[VERSIONING] End", zap.Error(err), zap.Int64("written", written))

	output := input
	output.AppendOutput(&jobs.ActionOutput{
		Success:true,
		StringBody:"Created new version " + resp.Version.Uuid,
	})

	return output, nil
}