package search

import (
	"encoding/json"

	"go.uber.org/zap"

	"github.com/micro/go-micro"
	api "github.com/micro/micro/api/proto"
	"github.com/pydio/services/common"
	"github.com/pydio/services/common/log"
	"github.com/pydio/services/common/proto/tree"
	"github.com/pydio/services/common/service"

	"strconv"
	"strings"

	"github.com/micro/cli"
	"github.com/pydio/services/common/views"
	"golang.org/x/net/context"
)

type Search struct {
	SearchClient tree.SearcherClient
	Router       *views.Router
}

type OutputNode struct {
	tree.Node
	Meta map[string]interface{}
}

func searchBuilder(service micro.Service) interface{} {
	return &Search{
		SearchClient: tree.NewSearcherClient(common.SERVICE_SEARCH, service.Client()),
		Router:       views.NewStandardRouter(false, true),
	}
}

// Parse parameters, from Get or Post
func (s *Search) parseParameters(params map[string]*api.Pair, query *tree.Query, showDetails bool, offset int32, limit int32) (bool, int32, int32) {

	if q, qOk := params["q"]; qOk {
		queryString := strings.Join(q.Values, "")
		if strings.Contains(queryString, ":") {
			query.FreeString = queryString
		} else {
			query.FileName = queryString
		}
	}
	if nodeType, ok := params["type"]; ok {
		ntValue := strings.Join(nodeType.Values, "")
		if ntValue == "file" {
			query.Type = 1
		} else if ntValue == "folder" {
			query.Type = 2
		}
	}

	if maxsize, ok := params["maxfilesize"]; ok {
		maxsizeValue, _ := strconv.ParseInt(strings.Join(maxsize.Values, ""), 10, 64)
		query.MaxSize = maxsizeValue
	}

	if minsize, ok := params["minfilesize"]; ok {
		minsizeValue, _ := strconv.ParseInt(strings.Join(minsize.Values, ""), 10, 64)
		query.MinSize = minsizeValue
	}

	if maxdate, ok := params["maxdate"]; ok {
		maxdateValue, _ := strconv.ParseInt(strings.Join(maxdate.Values, ""), 10, 64)
		query.MaxDate = maxdateValue
	}

	if mindate, ok := params["mindate"]; ok {
		mindateValue, _ := strconv.ParseInt(strings.Join(mindate.Values, ""), 10, 64)
		query.MinDate = mindateValue
	}

	if prefix, ok := params["prefix"]; ok {
		query.PathPrefix = append(query.PathPrefix, strings.Join(prefix.Values, ""))
	}

	if ext, ok := params["extension"]; ok {
		query.Extension = "." + strings.TrimLeft(strings.Join(ext.Values, ""), ".")
	}

	detail, dOk := params["detail"]
	if dOk && strings.Join(detail.Values, "") == "true" {
		showDetails = true
	}

	from, fOk := params["from"]
	size, sOk := params["size"]
	if fOk {
		i64, e := strconv.ParseInt(strings.Join(from.Values, ""), 10, 32)
		if e == nil {
			offset = int32(i64)
		}
	}
	if sOk {
		i64, er := strconv.ParseInt(strings.Join(size.Values, ""), 10, 32)
		if er == nil {
			limit = int32(i64)
		}
	}

	return showDetails, offset, limit
}

// Transform api.Request to grpc call to the Search microservice
func (s *Search) Query(ctx context.Context, req *api.Request, rsp *api.Response) error {

	query := &tree.Query{}
	details := false
	offset := int32(0)
	limit := int32(10)

	details, offset, limit = s.parseParameters(req.GetGet(), query, details, offset, limit)
	details, offset, limit = s.parseParameters(req.GetPost(), query, details, offset, limit)

	var nodes []*tree.Node
	prefixes := []string{}
	nodesPrefixes := map[string]string{}
	var passedPrefix string
	var passedWorkspaceSlug string
	if len(query.PathPrefix) > 0 {
		passedPrefix = strings.Trim(query.PathPrefix[0], "/")
		if len(strings.Split(passedPrefix, "/")) == 1 {
			passedWorkspaceSlug = passedPrefix
			passedPrefix = ""
		}
	}

	err := s.Router.WrapCallback(func(inputFilter views.NodeFilter, outputFilter views.NodeFilter) error {

		if len(passedPrefix) > 0 {
			// Passed prefix
			prefixes = append(prefixes, passedPrefix)

		} else {
			// Fill a context with current user info
			// (Let inputFilter apply the various necessary middlewares).
			loaderCtx, _ := inputFilter(ctx, &tree.Node{Path: ""}, "tmp")
			userWorkspaces := views.UserWorkspacesFromContext(loaderCtx)
			for _, w := range userWorkspaces {
				if len(passedWorkspaceSlug) > 0 && w.Slug != passedWorkspaceSlug {
					continue
				}
				if len(w.RootNodes) > 1 {
					for _, root := range w.RootNodes {
						prefixes = append(prefixes, w.Slug+"/"+root)
					}
				} else {
					prefixes = append(prefixes, w.Slug)
				}
			}
		}
		query.PathPrefix = []string{}

		var e error
		for _, p := range prefixes {
			rootNode := &tree.Node{Path: p}
			ctx, e = inputFilter(ctx, rootNode, "search-"+p)
			if e != nil {
				return e
			}
			log.Logger(ctx).Info("Filtered Node & Context", zap.String("path", rootNode.Path))
			nodesPrefixes[rootNode.Path] = p
			query.PathPrefix = append(query.PathPrefix, rootNode.Path)
		}

		sClient, err := s.SearchClient.Search(ctx, &tree.SearchRequest{
			Query:   query,
			Details: details,
			From:    offset,
			Size:    limit,
		})

		if err != nil {
			return err
		}

		defer sClient.Close()

		for {
			resp, rErr := sClient.Recv()
			if resp == nil {
				break
			} else if rErr != nil {
				return err
			}
			respNode := resp.Node
			for r, p := range nodesPrefixes {
				if strings.HasPrefix(respNode.Path, r) {
					_, err := outputFilter(ctx, respNode, "search-"+p)
					log.Logger(ctx).Debug("Response", zap.String("node", respNode.Path))
					if err != nil {
						return err
					}
					nodes = append(nodes, respNode)
				}
			}
		}
		return nil

	})

	if err != nil {
		log.Logger(ctx).Error("Query", zap.Error(err))
		return err
	}

	rsp.StatusCode = 200
	rsp.Header = make(map[string]*api.Pair, 1)
	rsp.Header["Content-type"] = &api.Pair{
		Key:    "Content-type",
		Values: []string{"application/json; charset=utf8"},
	}
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
	rsp.Body = string(b)

	return nil
}

// Starts the API
// Then Start micro --client=grpc api --namespace="pydio.service.api"
// Then call e.g. http://localhost:8080/search/query?q=filename"
func NewSearchApiService(ctx *cli.Context) (micro.Service, error) {

	srv := service.NewAPIService(searchBuilder, micro.Name(common.SERVICE_API_NAMESPACE_+"search"))
	return srv, nil

}
