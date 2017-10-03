package tree

import (
	"github.com/micro/go-micro/client"
	"github.com/micro/go-micro/errors"
	"github.com/pydio/services/common"
	"github.com/pydio/services/common/proto/jobs"
	"github.com/pydio/services/common/proto/tree"
	"github.com/pydio/services/common/views"
	"golang.org/x/net/context"
	"strconv"
	"strings"
	"github.com/pydio/services/common/log"
	"go.uber.org/zap"
	"fmt"
)

type CopyMoveAction struct {
	Client            views.Handler
	Move              bool
	Recursive		  bool
	TargetPlaceholder string
	CreateFolder      bool
}

var (
	copyMoveActionName = "actions.tree.copymove"
)

// Unique identifier
func (c *CopyMoveAction) GetName() string {
	return copyMoveActionName
}

// Pass parameters
func (c *CopyMoveAction) Init(job *jobs.Job, cl client.Client, action *jobs.Action) error {

	c.Client = views.NewStandardRouter(true, false)

	if action.Parameters == nil {
		return errors.InternalServerError(common.SERVICE_JOBS, "Could not find parameters for CopyMove action")
	}
	var tOk bool
	c.TargetPlaceholder, tOk = action.Parameters["target"]
	if !tOk {
		return errors.InternalServerError(common.SERVICE_JOBS, "Could not find parameters for CopyMove action")
	}
	c.Move = false
	if actionType, ok := action.Parameters["type"]; ok && actionType == "move" {
		c.Move = true
	}

	if createParam, ok := action.Parameters["create"]; ok {
		c.CreateFolder, _ = strconv.ParseBool(createParam)
	}

	if recurseParam, ok := action.Parameters["recursive"]; ok {
		c.Recursive, _ = strconv.ParseBool(recurseParam)
	}

	return nil
}

// Run the actual action code
func (c *CopyMoveAction) Run(ctx context.Context, input jobs.ActionMessage) (jobs.ActionMessage, error) {

	if len(input.Nodes) == 0 {
		return input.WithIgnore(), nil // Ignore
	}

	targetNode := &tree.Node{
		Path: c.TargetPlaceholder,
	}


	if targetNode.Path == input.Nodes[0].Path {
		// Do not copy on itself, ignore
		return input, nil
	}
	sourceNode := input.Nodes[0]

	readR, readE := c.Client.ReadNode(ctx, &tree.ReadNodeRequest{Node: sourceNode})
	if readE != nil {
		log.Logger(ctx).Error("Read Source", zap.Error(readE))
		return input.WithError(readE), readE
	}
	sourceNode = readR.Node
	output := input
	childrenMoved := 0

	if c.Recursive && !sourceNode.IsLeaf() {
		prefixPathSrc := strings.TrimRight(sourceNode.Path, "/")
		prefixPathTarget := strings.TrimRight(targetNode.Path, "/")
		// List all children and move them all
		streamer, _ := c.Client.ListNodes(ctx, &tree.ListNodesRequest{
			Node: sourceNode,
			Recursive:true,
			FilterType:tree.NodeType_LEAF,
		})
		children := []*tree.Node{}
		defer streamer.Close()

		for {
			child, cE := streamer.Recv()
			if cE != nil {
				break
			}
			if child == nil {
				continue
			}
			children = append(children, child.Node)
		}

		if len(children) > 0 {
			log.Logger(ctx).Info(fmt.Sprintf("There are %v children to move", len(children)))
		}

		for _, childNode := range children{

			childPath := childNode.Path
			targetPath := prefixPathTarget + "/" + strings.TrimPrefix(childPath, prefixPathSrc + "/")
			log.Logger(ctx).Info("Copy/Move " + childNode.Path + " to " + targetPath)

			_, e := c.Client.CopyObject(ctx, childNode, &tree.Node{ Path: targetPath }, &views.CopyRequestData{})
			if e != nil {
				log.Logger(ctx).Info("-- Copy ERROR", zap.Error(e), zap.Any("from", childNode.Path), zap.Any("to", targetPath))
				return output.WithError(e), e
			}
			log.Logger(ctx).Info("-- Copy Success")
			if c.Move {
				_, moveErr := c.Client.DeleteNode(ctx, &tree.DeleteNodeRequest{Node: childNode})
				if moveErr != nil {
					log.Logger(ctx).Info("-- Delete Error")
					return output.WithError(moveErr), moveErr
				}
				log.Logger(ctx).Info("-- Delete Success")
				output.AppendOutput(&jobs.ActionOutput{
					StringBody:"Moved file " + childPath + " to " + targetPath,
				})
			} else {
				output.AppendOutput(&jobs.ActionOutput{
					StringBody:"Copied file " + childPath + " to " + targetPath,
				})
			}
			childrenMoved ++

		}

	}

	if childrenMoved > 0 {
		log.Logger(ctx).Info(fmt.Sprintf("Successfully moved %v, now moving parent node", childrenMoved))
	}

	// Now Copy/Move initial node
	if sourceNode.IsLeaf() {
		_, e := c.Client.CopyObject(ctx, sourceNode, targetNode, &views.CopyRequestData{})
		if e != nil {
			return output.WithError(e), e
		}
		output = output.WithNode(targetNode)

	} else  if ! c.Move {
		// Folder copy: force recreating .__pydio hidden file. For move, it should have been moved around already
		_, e := c.Client.CreateNode(ctx, &tree.CreateNodeRequest{Node: targetNode})
		if e != nil {
			return output.WithError(e), e
		}
	}

	if c.Move {
		_, moveErr := c.Client.DeleteNode(ctx, &tree.DeleteNodeRequest{Node: sourceNode})
		if moveErr != nil {
			return output.WithError(moveErr), moveErr
		}
	}

	output.AppendOutput(&jobs.ActionOutput{
		Success: true,
	})

	return output, nil
}
