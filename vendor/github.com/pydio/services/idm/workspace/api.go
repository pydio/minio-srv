package workspace

import (
	"bytes"
	"encoding/json"
	"fmt"

	"go.uber.org/zap"

	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/any"
	"github.com/micro/go-micro"
	"github.com/micro/micro/api/proto"

	"github.com/micro/cli"
	"github.com/pydio/services/common"
	"github.com/pydio/services/common/log"
	"github.com/pydio/services/common/proto/idm"
	"github.com/pydio/services/common/service"
	serviceproto "github.com/pydio/services/common/service/proto"
	"golang.org/x/net/context"
)

type Workspace struct {
	client idm.WorkspaceServiceClient
}

func workspaceBuilder(service micro.Service) interface{} {
	return &Workspace{
		client: idm.NewWorkspaceServiceClient(common.SERVICE_WORKSPACE, service.Client()),
	}
}

func (s *Workspace) Put(ctx context.Context, req *go_micro_api.Request, rsp *go_micro_api.Response) error {

	workspace := new(idm.Workspace)
	err := json.Unmarshal([]byte(req.Body), &workspace)
	if err != nil {
		log.Logger(ctx).Error("Unmarshal", zap.Error(err))
		return err
	}

	log.Logger(ctx).Info("Put", zap.Any("workspace", workspace))

	response, er := s.client.CreateWorkspace(ctx, &idm.CreateWorkspaceRequest{
		Workspace: workspace,
	})

	if er != nil {
		return nil
	}

	rsp.StatusCode = 200
	rsp.Body = fmt.Sprintf(`{"success": true, "uuid": "%s"}`, response.GetWorkspace().GetUUID())

	return nil
}

func (s *Workspace) Search(ctx context.Context, req *go_micro_api.Request, rsp *go_micro_api.Response) error {

	var singleQueries []*idm.WorkspaceSingleQuery

	err := json.Unmarshal([]byte(req.Body), &singleQueries)
	if err != nil {
		log.Logger(ctx).Error("Failed to unmarshal workspace", zap.Error(err))
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

	stream, err := s.client.SearchWorkspace(ctx, &idm.SearchWorkspaceRequest{
		Query: &serviceproto.Query{
			SubQueries: queries,
		},
	})

	if err != nil {
		return err
	}

	defer stream.Close()

	var workspaces []*idm.Workspace
	for {
		response, err := stream.Recv()
		if err != nil {
			break
		}

		workspaces = append(workspaces, response.Workspace)
	}

	rsp.StatusCode = 200

	buf := new(bytes.Buffer)

	enc := json.NewEncoder(buf)
	d := map[string]interface{}{"success": true, "workspaces": workspaces}
	enc.Encode(d)

	rsp.Body = buf.String()

	return nil
}

// Starts the API
func NewAPIService(ctx *cli.Context) (micro.Service, error) {

	srv := service.NewAPIService(workspaceBuilder, micro.Name(common.SERVICE_API_NAMESPACE_+"workspace"))

	return srv, nil

}
