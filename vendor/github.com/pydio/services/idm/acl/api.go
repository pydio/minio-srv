package acl

import (
	"encoding/json"
	"fmt"

	"go.uber.org/zap"

	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/any"
	"github.com/micro/go-micro"
	"github.com/micro/micro/api/proto"
	"github.com/pydio/services/common"
	"github.com/pydio/services/common/log"
	"github.com/pydio/services/common/proto/acl"
	"github.com/pydio/services/common/service"
	serviceproto "github.com/pydio/services/common/service/proto"

	"github.com/micro/cli"
	"golang.org/x/net/context"
)

type Acl struct {
	client acl.ACLServiceClient
}

func aclBuilder(service micro.Service) interface{} {
	return &Acl{
		client: acl.NewACLServiceClient(common.SERVICE_ACL, service.Client()),
	}
}

func (s *Acl) Put(ctx context.Context, req *go_micro_api.Request, rsp *go_micro_api.Response) error {

	value := new(acl.ACL)
	err := json.Unmarshal([]byte(req.Body), &value)
	if err != nil {
		return err
	}

	_, err = s.client.CreateACL(ctx, &acl.CreateACLRequest{
		ACL: value,
	})
	if err != nil {
		log.Logger(ctx).Error("Put", zap.Error(err))
		return err
	}
	rsp.StatusCode = 200
	rsp.Body = `{"Success":"True"}`

	return nil
}

func (s *Acl) Delete(ctx context.Context, req *go_micro_api.Request, rsp *go_micro_api.Response) error {

	value := new(acl.ACL)

	err := json.Unmarshal([]byte(req.Body), &value)
	if err != nil {
		log.Logger(ctx).Error("Unmarshalling error ", zap.Any("body", req.Body), zap.Error(err))
		return err
	}

	query, _ := ptypes.MarshalAny(&acl.ACLSingleQuery{
		Actions:      []*acl.ACLAction{value.GetAction()},
		RoleIDs:      []string{value.GetRoleID()},
		WorkspaceIDs: []string{value.GetWorkspaceID()},
		NodeIDs:      []string{value.GetNodeID()},
	})

	if _, err := s.client.DeleteACL(ctx, &acl.DeleteACLRequest{
		Query: &serviceproto.Query{
			SubQueries: []*any.Any{query},
		},
	}); err != nil {
		log.Logger(ctx).Error("Deleting acl error ", zap.Error(err))
		return fmt.Errorf("Could not delete acl")
	}

	rsp.StatusCode = 200
	rsp.Body = `{"Success":"True"}`

	return nil
}

func (s *Acl) Search(ctx context.Context, req *go_micro_api.Request, rsp *go_micro_api.Response) error {

	data := req.GetGet()

	var aclActions []*acl.ACLAction

	var actions, roleIDs, nodeIDs, workspaceIDs []string

	if args, ok := data["action"]; ok {
		actions = args.Values
	}

	if args, ok := data["role_id"]; ok {
		roleIDs = args.Values
	}

	if args, ok := data["node_id"]; ok {
		nodeIDs = args.Values
	}

	if args, ok := data["workspace_id"]; ok {
		workspaceIDs = args.Values
	}

	for _, action := range actions {
		aclActions = append(aclActions, &acl.ACLAction{
			Name:  action,
			Value: "1",
		})
	}

	query, _ := ptypes.MarshalAny(&acl.ACLSingleQuery{
		Actions:      aclActions,
		RoleIDs:      roleIDs,
		WorkspaceIDs: workspaceIDs,
		NodeIDs:      nodeIDs,
	})

	stream, err := s.client.SearchACL(ctx, &acl.SearchACLRequest{
		Query: &serviceproto.Query{
			SubQueries: []*any.Any{query},
		},
	})

	if err != nil {
		return fmt.Errorf("Could not search acls")
	}

	defer stream.Close()

	rsp.StatusCode = 200
	rsp.Header = make(map[string]*go_micro_api.Pair, 1)
	rsp.Header["Content-type"] = &go_micro_api.Pair{
		Key:    "Content-type",
		Values: []string{"application/json; charset=utf8"},
	}

	var acls []*acl.ACL

	for {
		response, err := stream.Recv()

		if err != nil {
			break
		}

		acls = append(acls, response.GetACL())
	}

	b, _ := json.Marshal(acls)
	rsp.Body = string(b)

	return nil
}

func NewAPIService(ctx *cli.Context) (micro.Service, error) {

	srv := service.NewAPIService(aclBuilder, micro.Name(common.SERVICE_API_NAMESPACE_+"acl"))

	return srv, nil

}
