package tree

import (
	"github.com/micro/go-micro/client"
	"github.com/pydio/services/common/proto/jobs"
	"github.com/pydio/services/common/proto/tree"
	"github.com/pydio/services/common/views"
	"golang.org/x/net/context"
)

type DeleteAction struct {
	Client views.Handler
}

var (
	deleteActionName = "actions.tree.delete"
)

// Unique identifier
func (c *DeleteAction) GetName() string {
	return deleteActionName
}

// Pass parameters
func (c *DeleteAction) Init(job *jobs.Job, cl client.Client, action *jobs.Action) error {

	c.Client = views.NewStandardRouter(true, false)

	return nil
}

// Run the actual action code
func (c *DeleteAction) Run(ctx context.Context, input jobs.ActionMessage) (jobs.ActionMessage, error) {

	if len(input.Nodes) == 0 {
		return input.WithIgnore(), nil // Ignore
	}

	_, err := c.Client.DeleteNode(ctx, &tree.DeleteNodeRequest{Node: input.Nodes[0]})
	if err != nil {
		return input.WithError(err), err
	}

	output := input.WithNode(nil)
	output.AppendOutput(&jobs.ActionOutput{
		Success:    true,
		StringBody: "Deleted node",
	})

	return output, nil
}
