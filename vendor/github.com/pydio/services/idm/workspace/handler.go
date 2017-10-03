package workspace

import (
	"github.com/micro/go-micro/errors"
	"github.com/pydio/services/common"
	"github.com/pydio/services/common/proto/idm"
	"github.com/pydio/services/common/service/context"
	"golang.org/x/net/context"
)

// Handler definition
type Handler struct{}

// CreateWorkspace in database
func (h *Handler) CreateWorkspace(ctx context.Context, req *idm.CreateWorkspaceRequest, resp *idm.CreateWorkspaceResponse) error {
	dao := servicecontext.GetDAO(ctx).(DAO)

	err := dao.Add(req.Workspace)

	resp.Workspace = req.Workspace
	return err
}

// DeleteWorkspace from database
func (h *Handler) DeleteWorkspace(ctx context.Context, req *idm.DeleteWorkspaceRequest, response *idm.DeleteWorkspaceResponse) error {
	dao := servicecontext.GetDAO(ctx).(DAO)

	numRows, err := dao.Del(req.Query)
	response.RowsDeleted = numRows
	return err
}

// SearchWorkspace in database
func (h *Handler) SearchWorkspace(ctx context.Context, request *idm.SearchWorkspaceRequest, response idm.WorkspaceService_SearchWorkspaceStream) error {
	dao := servicecontext.GetDAO(ctx).(DAO)


	workspaces := new([]interface{})
	if err := dao.Search(request.Query, workspaces); err != nil {
		return err
	}

	for _, in := range *workspaces {
		workspace, ok := in.(*idm.Workspace)
		if !ok {
			return errors.InternalServerError(common.SERVICE_ROLE, "Wrong type")
		}

		response.Send(&idm.SearchWorkspaceResponse{Workspace: workspace})
	}

	response.Close()

	return nil
}

// StreamWorkspace from database
func (h *Handler) StreamWorkspace(ctx context.Context, streamer idm.WorkspaceService_StreamWorkspaceStream) error {
	dao := servicecontext.GetDAO(ctx).(DAO)

	defer streamer.Close()

	for {
		incoming, err := streamer.Recv()
		if incoming == nil || err != nil {
			break
		}

		workspaces := new([]interface{})
		if err := dao.Search(incoming.Query, workspaces); err != nil {
			return err
		}

		for _, in := range *workspaces {
			if workspace, ok := in.(*idm.Workspace); ok {
				streamer.Send(&idm.SearchWorkspaceResponse{Workspace: workspace})
			}
		}

		streamer.Send(nil)
	}

	return nil
}
