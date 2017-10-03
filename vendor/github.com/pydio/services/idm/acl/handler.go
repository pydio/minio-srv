package acl

import (
	"github.com/micro/go-micro/errors"
	"github.com/pydio/services/common"
	"github.com/pydio/services/common/proto/acl"
	"github.com/pydio/services/common/service/context"
	"golang.org/x/net/context"
)

// Handler definition
type Handler struct{}

// CreateACL in database
func (h *Handler) CreateACL(ctx context.Context, req *acl.CreateACLRequest, resp *acl.CreateACLResponse) error {

	dao := servicecontext.GetDAO(ctx).(DAO)

	if err := dao.Add(req.ACL); err != nil {
		return err
	}

	resp.ACL = req.ACL
	return nil
}

// DeleteACL from database
func (h *Handler) DeleteACL(ctx context.Context, req *acl.DeleteACLRequest, response *acl.DeleteACLResponse) error {

	dao := servicecontext.GetDAO(ctx).(DAO)

	numRows, err := dao.Del(req.Query)
	response.RowsDeleted = numRows
	return err
}

// SearchACL in database
func (h *Handler) SearchACL(ctx context.Context, request *acl.SearchACLRequest, response acl.ACLService_SearchACLStream) error {

	dao := servicecontext.GetDAO(ctx).(DAO)

	acls := new([]interface{})
	if err := dao.Search(request.Query, acls); err != nil {
		return err
	}

	for _, in := range *acls {
		val, ok := in.(*acl.ACL)
		if !ok {
			return errors.InternalServerError(common.SERVICE_ROLE, "Wrong type")
		}

		response.Send(&acl.SearchACLResponse{ACL: val})
	}

	response.Close()

	return nil
}

// StreamACL from database
func (h *Handler) StreamACL(ctx context.Context, streamer acl.ACLService_StreamACLStream) error {

	dao := servicecontext.GetDAO(ctx).(DAO)

	defer streamer.Close()

	for {
		incoming, err := streamer.Recv()
		if incoming == nil || err != nil {
			break
		}

		acls := new([]interface{})
		if err := dao.Search(incoming.Query, acls); err != nil {
			return err
		}

		for _, in := range *acls {
			if val, ok := in.(*acl.ACL); ok {
				streamer.Send(&acl.SearchACLResponse{ACL: val})
			}
		}

		streamer.Send(nil)
	}

	return nil
}
