package sync

import (
	"encoding/json"

	"github.com/pydio/poc/sync/common"
	"github.com/pydio/poc/sync/task"
	protosync "github.com/pydio/services/common/proto/sync"
	"github.com/pydio/services/common/proto/tree"
	"golang.org/x/net/context"
)

// Handler structure
type Handler struct {
	IndexClient tree.NodeProviderClient
	S3client    common.PathSyncTarget
	SyncTask    *task.Sync
}

// CreateNode Forwards to Index
func (s *Handler) CreateNode(ctx context.Context, req *tree.CreateNodeRequest, resp *tree.CreateNodeResponse) error {

	e := s.S3client.CreateNode(ctx, req.Node, req.UpdateIfExists)
	if e != nil {
		return e
	}
	resp.Node = req.Node
	return nil
}

// UpdateNode Forwards to S3
func (s *Handler) UpdateNode(ctx context.Context, req *tree.UpdateNodeRequest, resp *tree.UpdateNodeResponse) error {

	e := s.S3client.MoveNode(ctx, req.From.Path, req.To.Path)
	if e != nil {
		resp.Success = false
		return e
	}
	resp.Success = true
	return nil
}

// DeleteNode Forwards to S3
func (s *Handler) DeleteNode(ctx context.Context, req *tree.DeleteNodeRequest, resp *tree.DeleteNodeResponse) error {

	e := s.S3client.DeleteNode(ctx, req.Node.Path)
	if e != nil {
		resp.Success = false
		return e
	}
	resp.Success = true
	return nil
}

// ReadNode Forwards to Index
func (s *Handler) ReadNode(ctx context.Context, req *tree.ReadNodeRequest, resp *tree.ReadNodeResponse) error {

	r, e := s.IndexClient.ReadNode(ctx, req)
	if e != nil {
		return e
	}
	resp.Success = true
	resp.Node = r.Node
	return nil

}

// ListNodes Forward to index
func (s *Handler) ListNodes(ctx context.Context, req *tree.ListNodesRequest, resp tree.NodeProvider_ListNodesStream) error {

	client, e := s.IndexClient.ListNodes(ctx, req)
	if e != nil {
		return e
	}
	defer client.Close()
	for {
		nodeResp, re := client.Recv()
		if nodeResp == nil {
			break
		}
		if re != nil {
			return e
		}
		se := resp.Send(nodeResp)
		if se != nil {
			return e
		}
	}

	return nil
}

// TriggerResync sets 2 servers in sync
func (s *Handler) TriggerResync(c context.Context, req *protosync.ResyncRequest, resp *protosync.ResyncResponse) error {
	diff, e := s.SyncTask.Resync(req.DryRun)
	if e != nil {
		return e
	}
	data, e := json.Marshal(diff)
	if e != nil {
		return e
	}

	resp.Success = true
	resp.JsonDiff = string(data)
	return nil
}
