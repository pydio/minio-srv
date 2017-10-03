package tree

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
	"golang.org/x/net/context"
)

type Tree struct {
	TreeClient tree.NodeProviderClient
}

func treeBuilder(service micro.Service) interface{} {
	return &Tree{
		TreeClient: tree.NewNodeProviderClient(common.SERVICE_TREE, service.Client()),
	}
}

type OutputNode struct {
	tree.Node
	Meta map[string]interface{}
}

// List send a ListNodes request to the tree service
func (s *Tree) List(ctx context.Context, req *api.Request, rsp *api.Response) error {

	log.Logger(ctx).Info("List")

	path, pOk := req.Get["path"]
	if !pOk {
		return errors.BadRequest("pydio.service.api.tree", "Please provide a path parameter for query")
	}

	var Recursive bool
	recurse, dOk := req.Get["recursive"]
	if dOk && strings.Join(recurse.Values, "") == "true" {
		Recursive = true
	}

	client, err := s.TreeClient.ListNodes(ctx, &tree.ListNodesRequest{
		Node: &tree.Node{
			Path: strings.Join(path.Values, ""),
		},
		Recursive: Recursive,
	})

	if err != nil {
		log.Logger(ctx).Error("Error listing nodes", zap.Error(err))
		return err
	}

	defer client.Close()

	var nodes []*tree.Node
	for {
		resp, rErr := client.Recv()
		if resp == nil {
			break
		} else if rErr != nil {
			return err
		}
		nodes = append(nodes, resp.Node)
	}

	rsp.StatusCode = 200
	var b []byte
	if len(nodes) == 0 {
		b, _ = json.Marshal(struct {
			Message string
		}{
			Message: "No results",
		})
	} else {
		var outputNodes []*OutputNode
		for _, node := range nodes {
			out := &OutputNode{Node: *node}
			out.Meta = node.AllMetaDeserialized()
			out.MetaStore = nil
			outputNodes = append(outputNodes, out)
		}
		b, _ = json.Marshal(outputNodes)
	}
	rsp.Header = make(map[string]*api.Pair, 1)
	rsp.Header["Content-type"] = &api.Pair{
		Key:    "Content-type",
		Values: []string{"application/json; charset=utf8"},
	}
	rsp.Body = string(b)

	return nil
}

func (s *Tree) Stat(ctx context.Context, req *api.Request, rsp *api.Response) error {

	log.Logger(ctx).Info("Stat")

	path, pOk := req.Get["path"]
	if !pOk {
		return errors.BadRequest("pydio.service.api.tree", "Please provide a path parameter for query")
	}

	/*
		var Details bool
		detail, dOk := req.Get["detail"]
		if dOk && strings.Join(detail.Values, "") == "true" {
			Details = true
		}
	*/

	response, err := s.TreeClient.ReadNode(ctx, &tree.ReadNodeRequest{
		Node: &tree.Node{
			Path: strings.Join(path.Values, ""),
		},
	})

	if err != nil {
		return err
	}

	rsp.StatusCode = 200
	var b []byte
	out := &OutputNode{Node: *response.Node}
	out.Meta = response.Node.AllMetaDeserialized()
	out.MetaStore = nil
	b, _ = json.Marshal(out)
	rsp.Header = make(map[string]*api.Pair, 1)
	rsp.Header["Content-type"] = &api.Pair{
		Key:    "Content-type",
		Values: []string{"application/json; charset=utf8"},
	}
	rsp.Body = string(b)

	return nil
}

func NewTreeApiService(ctx *cli.Context) (micro.Service, error) {

	srv := service.NewAPIService(treeBuilder, micro.Name(common.SERVICE_API_NAMESPACE_+"tree"))
	return srv, nil

}
