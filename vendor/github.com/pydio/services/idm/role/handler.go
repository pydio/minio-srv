package role

import (
	"github.com/micro/go-micro/errors"
	"github.com/pydio/services/common"
	"github.com/pydio/services/common/proto/idm"
	"github.com/pydio/services/common/service/context"
	"golang.org/x/net/context"
)

// Handler definition
type Handler struct{}

// CreateRole in database
func (h *Handler) CreateRole(ctx context.Context, req *idm.CreateRoleRequest, resp *idm.CreateRoleResponse) error {
	dao := servicecontext.GetDAO(ctx).(DAO)

	err := dao.Add(req.Role)

	resp.Role = req.Role
	return err
}

// DeleteRole from database
func (h *Handler) DeleteRole(ctx context.Context, req *idm.DeleteRoleRequest, response *idm.DeleteRoleResponse) error {
	dao := servicecontext.GetDAO(ctx).(DAO)

	numRows, err := dao.Del(req.Query)
	response.RowsDeleted = numRows
	return err
}

// SearchRole in database
func (h *Handler) SearchRole(ctx context.Context, request *idm.SearchRoleRequest, response idm.RoleService_SearchRoleStream) error {
	dao := servicecontext.GetDAO(ctx).(DAO)

	roles := new([]interface{})
	if err := dao.Search(request.Query, roles); err != nil {
		return err
	}

	for _, in := range *roles {
		role, ok := in.(*idm.Role)
		if !ok {
			return errors.InternalServerError(common.SERVICE_ROLE, "Wrong type")
		}

		response.Send(&idm.SearchRoleResponse{Role: role})
	}

	response.Close()

	return nil
}

// StreamRole from database
func (h *Handler) StreamRole(ctx context.Context, streamer idm.RoleService_StreamRoleStream) error {
	dao := servicecontext.GetDAO(ctx).(DAO)

	defer streamer.Close()

	for {
		incoming, err := streamer.Recv()
		if incoming == nil || err != nil {
			break
		}

		roles := new([]interface{})
		if err := dao.Search(incoming.Query, roles); err != nil {
			return err
		}

		for _, in := range *roles {
			if role, ok := in.(*idm.Role); ok {
				streamer.Send(&idm.SearchRoleResponse{Role: role})
			}
		}

		streamer.Send(nil)
	}

	return nil
}
