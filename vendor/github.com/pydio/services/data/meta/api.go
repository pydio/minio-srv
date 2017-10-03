package meta

import (
	"encoding/json"

	"go.uber.org/zap"

	"github.com/micro/go-micro"
	"github.com/micro/go-micro/errors"
	api "github.com/micro/micro/api/proto"
	"github.com/pydio/services/common"
	"github.com/pydio/services/common/log"
	"github.com/pydio/services/common/proto/tree"
	"github.com/pydio/services/common/service"

	"strings"

	"github.com/micro/cli"
	"github.com/pydio/services/common/views"
	"golang.org/x/net/context"
)

type Meta struct {
	ViewClient      *views.Router
	MetaClientRead  tree.NodeProviderClient
	MetaClientWrite tree.NodeReceiverClient
}

func metaBuilder(service micro.Service) interface{} {
	return &Meta{
		ViewClient:      views.NewStandardRouter(false, true),
		MetaClientRead:  tree.NewNodeProviderClient(common.SERVICE_META, service.Client()),
		MetaClientWrite: tree.NewNodeReceiverClient(common.SERVICE_META, service.Client()),
	}
}

func (s *Meta) Put(ctx context.Context, req *api.Request, rsp *api.Response) error {

	body := req.Body

	log.Logger(ctx).Debug("Put", zap.String("body", body))

	data := make(map[string]interface{})
	err := json.Unmarshal([]byte(body), &data)
	if err != nil {
		log.Logger(ctx).Error("Error unmarshalling", zap.Error(err))
		return err
	}
	node, err := s.loadNodeByUuidOrPath(ctx, req)
	if err != nil {
		return err
	}
	for k, v := range data {
		node.SetMeta(k, v)
	}
	er := s.ViewClient.WrapCallback(func(inputFilter views.NodeFilter, outputFilter views.NodeFilter) error {
		ctx, _ = inputFilter(ctx, node, "in")
		_, er := s.MetaClientWrite.UpdateNode(ctx, &tree.UpdateNodeRequest{From: node, To: node})
		return er
	})
	if er != nil {
		return er
	}
	rsp.StatusCode = 200
	rsp.Body = `{"Success":"True"}`

	return nil
}

func (s *Meta) Delete(ctx context.Context, req *api.Request, rsp *api.Response) error {

	node, err := s.loadNodeByUuidOrPath(ctx, req)
	if err != nil {
		return err
	}
	// Meta Key to delete
	key, ok := req.Get["key"]
	if !ok {
		return errors.BadRequest(common.SERVICE_META, "Please pass a meta key to delete", 500)
	}
	node.SetMeta(strings.Join(key.Values, ""), "")
	er := s.ViewClient.WrapCallback(func(inputFilter views.NodeFilter, outputFilter views.NodeFilter) error {
		ctx, _ = inputFilter(ctx, node, "in")
		_, er := s.MetaClientWrite.UpdateNode(ctx, &tree.UpdateNodeRequest{From: node, To: node})
		return er
	})
	if er != nil {
		return er
	}
	rsp.StatusCode = 200
	rsp.Body = `{"Success":"True"}`

	return nil
}

func (s *Meta) Read(ctx context.Context, req *api.Request, rsp *api.Response) error {

	log.Logger(ctx).Debug("Read")

	node, err := s.loadNodeByUuidOrPath(ctx, req)
	if err != nil {
		rsp.StatusCode = 200
		rsp.Body = "{}"
		return nil
	}

	rsp.StatusCode = 200
	rsp.Header = make(map[string]*api.Pair, 1)
	rsp.Header["Content-type"] = &api.Pair{
		Key:    "Content-type",
		Values: []string{"application/json; charset=utf8"},
	}

	meta := node.AllMetaDeserialized()
	node.LegacyMeta(meta)
	b, _ := json.Marshal(meta)
	rsp.Body = string(b)

	return nil
}

func (s *Meta) loadNodeByUuidOrPath(ctx context.Context, req *api.Request) (*tree.Node, error) {

	uuid, uuidok := req.Get["uuid"]
	nPath, pathok := req.Get["path"]
	var hasPath, hasUuid bool
	hasUuid = uuidok && len(uuid.Values) > 0
	hasPath = pathok && len(nPath.Values) > 0

	if !hasUuid && !hasPath {
		return nil, errors.BadRequest("pydio.service.api.meta", "Please provide at least an uuid or a path")
	}
	var response *tree.ReadNodeResponse
	var err error
	if hasUuid {
		log.Logger(ctx).Debug("Querying Meta Service by Uuid")

		response, err = s.MetaClientRead.ReadNode(ctx, &tree.ReadNodeRequest{
			Node: &tree.Node{
				Uuid: strings.Join(uuid.Values, ""),
			},
		})
	} else {
		log.Logger(ctx).Debug("Querying Tree Service by Path: ", zap.Any("values", nPath.Values))

		response, err = s.ViewClient.ReadNode(ctx, &tree.ReadNodeRequest{
			Node: &tree.Node{
				Path: strings.Join(nPath.Values, ""),
			},
		})
	}

	if err != nil {
		log.Logger(ctx).Error("loadNodeByUuidOrPath", zap.Error(err))
		return nil, err
	}
	return response.Node, nil

}

// Starts the API
// Then Start :
// micro --client=grpc api --namespace="pydio.service.api"
//
// Then call e.g. http://localhost:8080/meta/read?uuid="existing-uuid" or ?path="datasource/path/to/node"
func NewMetaApiService(ctx *cli.Context) (micro.Service, error) {

	srv := service.NewAPIService(metaBuilder, micro.Name(common.SERVICE_API_NAMESPACE_+"meta"))
	return srv, nil
}
