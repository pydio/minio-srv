package role

import (
	"bytes"
	"encoding/json"
	"fmt"

	"go.uber.org/zap"

	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/any"
	"github.com/micro/go-micro"
	"github.com/micro/micro/api/proto"
	"github.com/pydio/services/common"
	"github.com/pydio/services/common/log"
	"github.com/pydio/services/common/proto/idm"
	"github.com/pydio/services/common/service"
	serviceproto "github.com/pydio/services/common/service/proto"

	"github.com/micro/cli"
	"golang.org/x/net/context"
)

type Role struct {
	client idm.RoleServiceClient
}

func roleBuilder(service micro.Service) interface{} {

	return &Role{
		client: idm.NewRoleServiceClient(common.SERVICE_ROLE, service.Client()),
	}
}

func (s *Role) Put(ctx context.Context, req *go_micro_api.Request, rsp *go_micro_api.Response) error {

	role := new(idm.Role)
	err := json.Unmarshal([]byte(req.Body), &role)
	if err != nil {
		return err
	}

	response, er := s.client.CreateRole(ctx, &idm.CreateRoleRequest{
		Role: role,
	})

	if er != nil {
		return nil
	}

	rsp.StatusCode = 200
	rsp.Body = fmt.Sprintf(`{"success": true, "id": "%s"}`, response.GetRole().GetID())

	return nil
}

func (s *Role) Search(ctx context.Context, req *go_micro_api.Request, rsp *go_micro_api.Response) error {

	var singleQueries []*idm.RoleSingleQuery

	err := json.Unmarshal([]byte(req.Body), &singleQueries)
	if err != nil {
		log.Logger(ctx).Error("Failed to unmarshal role", zap.Error(err))
		return err
	}

	log.Logger(ctx).Info("Search", zap.Any("query", singleQueries))

	var queries []*any.Any
	for _, singleQuery := range singleQueries {
		query, err := ptypes.MarshalAny(singleQuery)
		if err != nil {
			return err
		}
		queries = append(queries, query)
	}

	stream, err := s.client.SearchRole(ctx, &idm.SearchRoleRequest{
		Query: &serviceproto.Query{
			SubQueries: queries,
		},
	})

	if err != nil {
		return err
	}

	defer stream.Close()

	var roles []*idm.Role
	for {
		response, err := stream.Recv()
		if err != nil {
			break
		}

		roles = append(roles, response.Role)
	}

	rsp.StatusCode = 200

	buf := new(bytes.Buffer)

	enc := json.NewEncoder(buf)
	d := map[string]interface{}{"success": true, "roles": roles}
	enc.Encode(d)

	rsp.Body = buf.String()

	return nil
}

// Starts the API
func NewAPIService(ctx *cli.Context) (micro.Service, error) {

	srv := service.NewAPIService(roleBuilder, micro.Name(common.SERVICE_API_NAMESPACE_+"role"))

	return srv, nil
}
