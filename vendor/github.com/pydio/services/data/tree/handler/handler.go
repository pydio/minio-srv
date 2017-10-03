package handler

import (
	"strings"
	"sync"

	"go.uber.org/zap"

	"github.com/micro/go-micro/errors"
	"golang.org/x/net/context"

	"github.com/pydio/services/common"
	"github.com/pydio/services/common/log"
	"github.com/pydio/services/common/proto/tree"
)

type DataSource struct {
	Reader tree.NodeProviderClient
	Writer tree.NodeReceiverClient
	S3URL  string
}

type TreeServer struct {
	DataSources         map[string]DataSource
	MetaServiceClient   tree.NodeProviderClient
	MetaServiceStreamer tree.NodeProviderStreamerClient
	ConfigsMutex        *sync.Mutex
}

func (s *TreeServer) treeNodeToDataSourcePath(node *tree.Node) (dataSourceName string, dataSourcePath string) {

	path := strings.Trim(node.GetPath(), "/")
	if path == "" {
		return "", ""
	}
	parts := strings.Split(path, "/")
	dataSourceName = parts[0]
	if len(parts) > 1 {
		dataSourcePath = strings.Join(parts[1:], "/")
	} else {
		dataSourcePath = ""
	}

	return dataSourceName, dataSourcePath

}

func (s *TreeServer) updateDataSourceNode(node *tree.Node, dataSourceName string) {

	dsPath := strings.TrimLeft(node.GetPath(), "/")
	newPath := dataSourceName + "/" + dsPath

	node.Path = newPath
	node.SetMeta(common.META_NAMESPACE_DATASOURCE_PATH, dsPath)
	if ds, ok := s.DataSources[dataSourceName]; ok {
		node.SetMeta(common.META_NAMESPACE_OBJECT_SERVICE, ds.S3URL)
	}
}

func (s *TreeServer) enrichNodeWithMeta(ctx context.Context, node *tree.Node) {

	metaResponse, metaErr := s.MetaServiceClient.ReadNode(ctx, &tree.ReadNodeRequest{
		Node: node,
	})

	if metaErr == nil {
		node.MetaStore = metaResponse.Node.MetaStore
	}

}

/* =============================================================================
 *  Server public Methods
 * ============================================================================ */

// CreateNode implementation for the TreeServer
func (s *TreeServer) CreateNode(ctx context.Context, req *tree.CreateNodeRequest, resp *tree.CreateNodeResponse) error {
	node := req.GetNode()

	log.Logger(ctx).Info("CreateNode", zap.Any("node", node))

	dsName, dsPath := s.treeNodeToDataSourcePath(node)
	if dsName == "" || dsPath == "" {
		return errors.Forbidden(common.SERVICE_TREE, "Cannot write to root node or to datasource node")
	}

	if ds, ok := s.DataSources[dsName]; ok {

		node.Path = dsPath
		req := &tree.CreateNodeRequest{Node: node}

		response, e := ds.Writer.CreateNode(ctx, req)
		if e != nil {
			return e
		}
		s.updateDataSourceNode(response.Node, dsName)
		resp.Node = response.Node

		return nil
	}

	return errors.Forbidden(dsName, "Unknown data source")
}

// ReadNode implementation for the TreeServer
func (s *TreeServer) ReadNode(ctx context.Context, req *tree.ReadNodeRequest, resp *tree.ReadNodeResponse) error {

	node := req.GetNode()
	if node.GetPath() == "" && node.GetUuid() != "" {

		log.Logger(ctx).Info("ReadNode", zap.String("uuid", node.GetUuid()))

		respNode, err := s.lookUpByUuid(ctx, node.GetUuid())
		if err != nil {
			return err
		}
		resp.Node = respNode

		log.Logger(ctx).Info("Response after lookUp", zap.String("path", resp.Node.GetPath()))

		s.enrichNodeWithMeta(ctx, resp.Node)

		return nil
	}

	dsName, dsPath := s.treeNodeToDataSourcePath(node)

	log.Logger(ctx).Info("ReadNode", zap.String("path", node.GetPath()))

	if dsName == "" && dsPath == "" {
		resp.Node = &tree.Node{Uuid: "ROOT", Path: "/"}
		return nil
	}

	if ds, ok := s.DataSources[dsName]; ok {

		req := &tree.ReadNodeRequest{
			Node: &tree.Node{Path: dsPath},
		}

		response, rErr := ds.Reader.ReadNode(ctx, req)

		if rErr != nil {
			return rErr
		}

		resp.Node = response.Node
		s.updateDataSourceNode(resp.Node, dsName)
		s.enrichNodeWithMeta(ctx, resp.Node)

		return nil
	}

	return errors.NotFound(node.GetPath(), "Not found")
}

func (s *TreeServer) ListNodes(ctx context.Context, req *tree.ListNodesRequest, resp tree.NodeProvider_ListNodesStream) error {

	// Special case to get ancestors
	if req.Ancestors {

		defer resp.Close()
		// FIRST FIND NODE & DS
		var dsName, dsPath string
		sendNode := req.Node

		log.Logger(ctx).Debug("Find Ancestors", zap.Any("node", req.Node))

		if req.Node.GetPath() == "" && req.Node.GetUuid() != "" {
			log.Logger(ctx).Debug("First Find node by uuid ", zap.String("uuid", req.Node.GetUuid()))

			sendNode, err := s.lookUpByUuid(ctx, req.Node.GetUuid())
			if err != nil {
				return err
			}
			dsName, dsPath = s.treeNodeToDataSourcePath(sendNode)
		} else {
			dsName, dsPath = s.treeNodeToDataSourcePath(req.Node)
		}
		if dsName == "" && dsPath == "" {
			// ROOT NODE
			return errors.BadRequest(common.SERVICE_TREE, "Cannot get ancestors on ROOT node!")

		} else if len(dsPath) > 0 {

			sendNode.Path = dsPath
			streamer, err := s.DataSources[dsName].Reader.ListNodes(ctx, &tree.ListNodesRequest{
				Node:      sendNode,
				Ancestors: true,
			})
			if err != nil {
				return errors.InternalServerError(common.SERVICE_TREE, "Cannot send List request to underlying datasource")
			}
			defer streamer.Close()
			for {
				listResponse, err := streamer.Recv()
				if listResponse == nil || err != nil {
					break
				}
				respNode := listResponse.Node
				s.updateDataSourceNode(respNode, dsName)
				if respNode.Uuid == "ROOT" {
					// Replace DataSource "ROOT" by current DataSource Node
					respNode.Uuid = "DATASOURCE:" + dsName
				}
				resp.Send(&tree.ListNodesResponse{
					Node: respNode,
				})
			}

		}

		// NOW SEND ROOT NODE
		resp.Send(&tree.ListNodesResponse{
			Node: &tree.Node{
				Uuid: "ROOT",
				Path: "/",
			},
		})

		return nil

	} else {

		var numberSent int64
		numberSent = 0
		return s.ListNodesWithLimit(ctx, req, resp, &numberSent)

	}

}

// ListNodesWithLimit implementation for the TreeServer
func (s *TreeServer) ListNodesWithLimit(ctx context.Context, req *tree.ListNodesRequest, resp tree.NodeProvider_ListNodesStream, numberSent *int64) error {

	defer resp.Close()

	node := req.GetNode()

	dsName, dsPath := s.treeNodeToDataSourcePath(node)
	limit := req.Limit

	checkLimit := func() bool {
		*numberSent++
		if limit == 0 {
			return false
		}
		if *numberSent >= limit {
			log.Logger(ctx).Warn("Breaking result at Limit", zap.Int64("limit", limit))
			return true
		}
		return false
	}

	if dsName == "" {

		for name := range s.DataSources {

			outputNode := &tree.Node{
				Uuid: "DATASOURCE:" + name,
				Path: name,
			}
			outputNode.SetMeta("name", name)
			log.Logger(ctx).Info("Listing datasources", zap.Any("node", outputNode))
			resp.Send(&tree.ListNodesResponse{
				Node: outputNode,
			})
			if req.Recursive {
				s.ListNodesWithLimit(ctx, &tree.ListNodesRequest{
					Node:      &tree.Node{Path: name},
					Recursive: true,
				}, resp, numberSent)
			}
			if checkLimit() {
				return nil
			}
		}
		return nil
	}

	if ds, ok := s.DataSources[dsName]; ok {

		log.Logger(ctx).Info("Listing nodes", zap.String("dsPath", dsPath), zap.String("dsName", dsName))

		req := &tree.ListNodesRequest{
			Node:       &tree.Node{Path: dsPath},
			Limit:      req.Limit,
			Recursive:  req.Recursive,
			FilterType: req.FilterType,
		}

		stream, _ := ds.Reader.ListNodes(ctx, req)
		defer stream.Close()

		var metaStreamer tree.NodeProviderStreamer_ReadNodeStreamClient
		if s.MetaServiceStreamer != nil {
			var metaE error
			metaStreamer, metaE = s.MetaServiceStreamer.ReadNodeStream(ctx)
			if metaE != nil {
				return metaE
			}
		}

		for {
			clientResponse, err := stream.Recv()

			if clientResponse == nil {
				break
			}

			if err != nil {
				break
			}

			// Send node to MetaClient (streaming)
			if s.MetaServiceStreamer != nil && 0 == 1 {

				sendError := metaStreamer.Send(&tree.ReadNodeRequest{
					Node: clientResponse.Node,
				})

				if sendError != nil {
					log.Logger(ctx).Error("Error while sending to metaStreamer", zap.Error(sendError))
				}

				metaResponse, err := metaStreamer.Recv()
				if err == nil && metaResponse != nil && metaResponse.Node != nil {
					s.updateDataSourceNode(metaResponse.Node, dsName)
					resp.Send(&tree.ListNodesResponse{
						Node: metaResponse.Node,
					})
					if checkLimit() {
						return nil
					} else {
						continue
					}
				}
			}

			s.updateDataSourceNode(clientResponse.Node, dsName)
			resp.Send(clientResponse)
			if checkLimit() {
				return nil
			}
		}

		if s.MetaServiceStreamer != nil {
			metaStreamer.Close()
		}
	}

	return errors.NotFound(node.GetPath(), "Not found")
}

// UpdateNode implementation for the TreeServer
func (s *TreeServer) UpdateNode(ctx context.Context, req *tree.UpdateNodeRequest, resp *tree.UpdateNodeResponse) error {

	from := req.GetFrom()
	to := req.GetTo()

	log.Logger(ctx).Info("UpdateNode", zap.Any("from", from), zap.Any("to", to))

	dsNameFrom, dsPathFrom := s.treeNodeToDataSourcePath(from)
	dsNameTo, dsPathTo := s.treeNodeToDataSourcePath(to)
	if dsNameFrom == "" || dsNameTo == "" || dsPathFrom == "" || dsPathTo == "" {
		return errors.Forbidden(common.SERVICE_TREE, "Cannot write to root node or to datasource node")
	}
	if dsNameFrom != dsNameTo {
		return errors.Forbidden(common.SERVICE_TREE, "Cannot move between two different datasources")
	}

	if ds, ok := s.DataSources[dsNameTo]; ok {

		from.Path = dsPathFrom
		to.Path = dsPathTo

		req := &tree.UpdateNodeRequest{From: from, To: to}

		response, _ := ds.Writer.UpdateNode(ctx, req)

		resp.Success = response.Success
		resp.Node = response.Node

		return nil
	}

	return errors.Forbidden(common.SERVICE_TREE, "Unknown data source")
}

// DeleteNode implementation for the TreeServer
func (s *TreeServer) DeleteNode(ctx context.Context, req *tree.DeleteNodeRequest, resp *tree.DeleteNodeResponse) error {
	node := req.GetNode()
	dsName, dsPath := s.treeNodeToDataSourcePath(node)
	if dsName == "" || dsPath == "" {
		return errors.Forbidden(common.SERVICE_TREE, "Cannot delete root node or datasource node")
	}

	if ds, ok := s.DataSources[dsName]; ok {

		node.Path = dsPath
		response, _ := ds.Writer.DeleteNode(ctx, &tree.DeleteNodeRequest{Node: node})
		resp.Success = response.Success

		return nil
	}

	return errors.Forbidden(common.SERVICE_TREE, "Unknown data source")
}

func (s *TreeServer) lookUpByUuid(ctx context.Context, uuid string) (*tree.Node, error) {

	c, cancel := context.WithCancel(ctx)
	var foundNode *tree.Node

	wg := &sync.WaitGroup{}
	for dsName, ds := range s.DataSources {
		dataSource := dsName
		currentClient := ds.Reader
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := currentClient.ReadNode(c, &tree.ReadNodeRequest{Node: &tree.Node{Uuid: uuid}})
			if err == nil && resp.Node != nil {
				s.updateDataSourceNode(resp.Node, dataSource)

				log.Logger(ctx).Info("[Look Up] Found node", zap.String("uuid", resp.Node.Uuid), zap.String("datasource", dataSource))
				foundNode = resp.Node
				cancel()
			}
		}()
	}

	wg.Wait()
	if foundNode != nil {
		return foundNode, nil
	} else {
		return nil, errors.NotFound(uuid, "Not found")
	}

}
