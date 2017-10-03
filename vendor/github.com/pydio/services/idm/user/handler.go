package user

import (
	"github.com/micro/go-micro/errors"
	"github.com/pydio/services/common"
	"github.com/pydio/services/common/proto/idm"
	"github.com/pydio/services/common/service/context"
	"golang.org/x/net/context"
)

// Handler definition
type Handler struct{}

// CreateUser in database
func (h *Handler) CreateUser(ctx context.Context, req *idm.CreateUserRequest, resp *idm.CreateUserResponse) error {
	dao := servicecontext.GetDAO(ctx).(DAO)

	err := dao.Add(req.User)

	resp.User = req.User
	return err
}

// Bind user with login/password
func (h *Handler) BindUser(ctx context.Context, req *idm.BindUserRequest, resp *idm.BindUserResponse) error {
	dao := servicecontext.GetDAO(ctx).(DAO)

	user, err := dao.Bind(req.UserName, req.Password)
	if err != nil {
		return err
	}
	resp.User = user
	return nil

}

// DeleteUser from database
func (h *Handler) DeleteUser(ctx context.Context, req *idm.DeleteUserRequest, response *idm.DeleteUserResponse) error {
	dao := servicecontext.GetDAO(ctx).(DAO)

	numRows, err := dao.Del(req.Query)
	response.RowsDeleted = numRows
	return err
}

// SearchUser in database
func (h *Handler) SearchUser(ctx context.Context, request *idm.SearchUserRequest, response idm.UserService_SearchUserStream) error {
	dao := servicecontext.GetDAO(ctx).(DAO)

	users := new([]interface{})
	if err := dao.Search(request.Query, users); err != nil {
		return err
	}

	for _, in := range *users {
		user, ok := in.(*idm.User)
		if !ok {
			return errors.InternalServerError(common.SERVICE_USER, "Wrong type")
		}

		response.Send(&idm.SearchUserResponse{User: user})
	}

	response.Close()

	return nil
}

// StreamUser from database
func (h *Handler) StreamUser(ctx context.Context, streamer idm.UserService_StreamUserStream) error {
	dao := servicecontext.GetDAO(ctx).(DAO)

	defer streamer.Close()

	for {
		incoming, err := streamer.Recv()
		if incoming == nil || err != nil {
			break
		}

		users := new([]interface{})
		if err := dao.Search(incoming.Query, users); err != nil {
			return err
		}

		for _, in := range *users {
			if user, ok := in.(*idm.User); ok {
				streamer.Send(&idm.SearchUserResponse{User: user})
			}
		}

		streamer.Send(nil)
	}

	return nil
}
