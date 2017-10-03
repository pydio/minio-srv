package tree

import (
	"github.com/micro/go-micro/client"
	"github.com/pydio/services/common"
	"github.com/pydio/services/common/proto/jobs"
	"github.com/pydio/services/common/proto/tree"
	"golang.org/x/net/context"
)

type MetaAction struct {
	Client        tree.NodeReceiverClient
	MetaNamespace string
	MetaValue     interface{}
}

var (
	metaActionName = "actions.tree.meta"
)

// Unique identifier
func (c *MetaAction) GetName() string {
	return metaActionName
}

// Pass parameters
func (c *MetaAction) Init(job *jobs.Job, cl client.Client, action *jobs.Action) error {

	c.Client = tree.NewNodeReceiverClient(common.SERVICE_META, cl)
	c.MetaNamespace = action.Parameters["metaName"]
	c.MetaValue = action.Parameters["metaValue"]

	return nil
}

// Run the actual action code
func (c *MetaAction) Run(ctx context.Context, input jobs.ActionMessage) (jobs.ActionMessage, error) {

	if len(input.Nodes) == 0 {
		return input.WithIgnore(), nil // Ignore
	}

	// Update Metadata
	input.Nodes[0].SetMeta(c.MetaNamespace, c.MetaValue)

	_, err := c.Client.UpdateNode(ctx, &tree.UpdateNodeRequest{From: input.Nodes[0], To: input.Nodes[0]})
	if err != nil {
		return input.WithError(err), err
	}

	input.AppendOutput(&jobs.ActionOutput{Success: true})

	return input, nil
}
