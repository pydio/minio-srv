package tree

import (
	"github.com/micro/go-micro/client"
	"github.com/pydio/services/common/proto/jobs"
	"github.com/pydio/services/common/proto/tree"
	"github.com/pydio/services/common/views"
	"golang.org/x/net/context"
	"github.com/pydio/services/common/config"
	"path/filepath"
	"io/ioutil"
	"github.com/pydio/services/common/log"
	"os"
	"encoding/json"
)

type SnapshotAction struct {
	Client views.Handler
	Target string
}

var (
	snapshotActionName = "actions.tree.snapshot"
)

// Unique identifier
func (c *SnapshotAction) GetName() string {
	return snapshotActionName
}

// Pass parameters
func (c *SnapshotAction) Init(job *jobs.Job, cl client.Client, action *jobs.Action) error {

	c.Client = views.NewStandardRouter(true, false)
	if target, ok := action.Parameters["target_file"]; ok {
		c.Target = target
	} else {
		tmpDir, _ := config.ApplicationDataDir()
		c.Target = filepath.Join(tmpDir, "snapshot.json")
	}

	return nil
}

// Run the actual action code
func (c *SnapshotAction) Run(ctx context.Context, input jobs.ActionMessage) (jobs.ActionMessage, error) {

	streamer, err := c.Client.ListNodes(ctx, &tree.ListNodesRequest{
		Node: &tree.Node{Path:"miniods1"},
		Recursive:true,
	})
	if err != nil {
		return input.WithError(err), err
	}
	defer streamer.Close()

	nodesList := []*tree.Node{}
	for {
		resp, e := streamer.Recv()
		if e != nil {
			break
		}
		if resp == nil {
			continue
		}
		nodesList = append(nodesList, resp.Node)
	}

	content, _ := json.Marshal(nodesList)
	os.Remove(c.Target)
	writeErr := ioutil.WriteFile(c.Target, []byte(content), 0755)
	if writeErr != nil {
		return input.WithError(writeErr), writeErr
	}

	log.Logger(ctx).Info("Tree snapshot written to " + c.Target,)
	output := input.WithNode(nil)
	output.AppendOutput(&jobs.ActionOutput{
		Success:    true,
		StringBody: "Tree snapshot written to " + c.Target,
	})

	return output, nil
}
