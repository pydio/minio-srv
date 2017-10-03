package meta

import (
	"go.uber.org/zap"

	"golang.org/x/net/context"

	"encoding/json"

	"github.com/micro/go-micro/client"
	"github.com/micro/go-micro/errors"
	"github.com/pydio/services/common"
	"github.com/pydio/services/common/event"
	"github.com/pydio/services/common/log"
	"github.com/pydio/services/common/proto/tree"
	"github.com/pydio/services/common/service/context"
)

// MetaServer definition
type MetaServer struct {
	//	Dao           DAO
	eventsChannel chan *event.EventWithContext
}

// CreateNodeChangeSubscriber that will treat events for the meta server
func (s *MetaServer) CreateNodeChangeSubscriber() *EventsSubscriber {

	if s.eventsChannel == nil {
		s.initEventsChannel()
	}
	subscriber := &EventsSubscriber{
		outputChannel: s.eventsChannel,
	}
	return subscriber
}

func (s *MetaServer) initEventsChannel() {

	// Todo: find a way to close channel on topic UnSubscription ?

	s.eventsChannel = make(chan *event.EventWithContext)
	go func() {
		for eventWCtx := range s.eventsChannel {
			s.processEvent(eventWCtx.Context, eventWCtx.Event)
		}
	}()
}

func (s *MetaServer) processEvent(ctx context.Context, e *tree.NodeChangeEvent) {

	log.Logger(ctx).Info("processEvent", zap.Any("type", e.GetType()))

	switch e.GetType() {
	case tree.NodeChangeEvent_CREATE:
		log.Logger(ctx).Info("Received Create event", zap.Any("event", e))

		// Let's extract the basic information from the tree and store it
		s.UpdateNode(ctx, &tree.UpdateNodeRequest{
			To: e.Target,
		}, &tree.UpdateNodeResponse{})
		break
	case tree.NodeChangeEvent_UPDATE_PATH:
		log.Logger(ctx).Info("Received Update event", zap.Any("event", e))

		// Let's extract the basic information from the tree and store it
		s.UpdateNode(ctx, &tree.UpdateNodeRequest{
			To: e.Target,
		}, &tree.UpdateNodeResponse{})
		break
	case tree.NodeChangeEvent_UPDATE_META:
		log.Logger(ctx).Info("Received Update meta", zap.Any("event", e))

		// Let's extract the basic information from the tree and store it
		s.UpdateNode(ctx, &tree.UpdateNodeRequest{
			To: e.Target,
		}, &tree.UpdateNodeResponse{})
		break
	case tree.NodeChangeEvent_UPDATE_CONTENT:
		// We may have to store the metadata again
		log.Logger(ctx).Info("Received Update content", zap.Any("event", e))
		break
	case tree.NodeChangeEvent_DELETE:
		// Lets delete all metadata
		log.Logger(ctx).Info("Received Delete content", zap.Any("event", e))

		s.DeleteNode(ctx, &tree.DeleteNodeRequest{
			Node: e.Source,
		}, &tree.DeleteNodeResponse{})
	default:
		log.Logger(ctx).Error("Could not recognize event type", zap.Any("event", e.GetType()))
	}
}

// ReadNode information off the meta server
func (s *MetaServer) ReadNode(ctx context.Context, req *tree.ReadNodeRequest, resp *tree.ReadNodeResponse) (err error) {
	dao := servicecontext.GetDAO(ctx).(DAO)

	if req.Node == nil || req.Node.Uuid == "" {
		return errors.BadRequest(common.SERVICE_META, "Please provide a Node with a Uuid")
	}
	metadata, err := dao.GetMetadata(req.Node.Uuid)
	if metadata == nil || err != nil {
		return errors.NotFound(common.SERVICE_META, "Node with Uuid "+req.Node.Uuid+" not found")
	}

	resp.Success = true
	respNode := req.Node
	for k, v := range metadata {
		var metaValue interface{}
		json.Unmarshal([]byte(v), &metaValue)
		respNode.SetMeta(k, metaValue)
	}
	resp.Node = respNode

	return nil
}

// ReadNode implementation as a bidirectional stream
func (s *MetaServer) ReadNodeStream(ctx context.Context, streamer tree.NodeProviderStreamer_ReadNodeStreamStream) error {

	defer streamer.Close()

	for {
		request, err := streamer.Recv()
		if request == nil {
			break
		}
		if err != nil {
			return err
		}
		response := &tree.ReadNodeResponse{}

		log.Logger(ctx).Debug("ReadNodeStream", zap.String("path", request.Node.Path))

		e := s.ReadNode(ctx, &tree.ReadNodeRequest{Node: request.Node}, response)
		if e != nil {
			if errors.Parse(e.Error()).Code == 404 {
				// There is no metadata, simply return the original node
				streamer.Send(&tree.ReadNodeResponse{Node: request.Node})
			} else {
				return e
			}
		} else {
			sendErr := streamer.Send(&tree.ReadNodeResponse{Node: response.Node})
			if sendErr != nil {
				return e
			}
		}
	}

	return nil

}

// ListNodes information from the meta server (Not implemented)
func (s *MetaServer) ListNodes(ctx context.Context, req *tree.ListNodesRequest, resp tree.NodeProvider_ListNodesStream) (err error) {
	return errors.BadRequest("ListNodes", "Method not implemented")
}

// CreateNode metadata
func (s *MetaServer) CreateNode(ctx context.Context, req *tree.CreateNodeRequest, resp *tree.CreateNodeResponse) (err error) {
	dao := servicecontext.GetDAO(ctx).(DAO)

	if err := dao.SetMetadata(req.Node.Uuid, req.Node.MetaStore); err != nil {
		resp.Success = false
	}

	resp.Success = true

	client.Publish(ctx, client.NewPublication(common.TOPIC_META_CHANGES, &tree.NodeChangeEvent{
		Type:   tree.NodeChangeEvent_UPDATE_META,
		Target: req.Node,
	}))

	return nil
}

// UpdateNode metadata
func (s *MetaServer) UpdateNode(ctx context.Context, req *tree.UpdateNodeRequest, resp *tree.UpdateNodeResponse) (err error) {

	dao := servicecontext.GetDAO(ctx).(DAO)

	if err := dao.SetMetadata(req.To.Uuid, req.To.MetaStore); err != nil {
		resp.Success = false
	}

	resp.Success = true

	client.Publish(ctx, client.NewPublication(common.TOPIC_META_CHANGES, &tree.NodeChangeEvent{
		Type:   tree.NodeChangeEvent_UPDATE_META,
		Target: req.To,
	}))

	return nil
}

// DeleteNode metadata (Not implemented)
func (s *MetaServer) DeleteNode(ctx context.Context, request *tree.DeleteNodeRequest, result *tree.DeleteNodeResponse) (err error) {

	// Delete all meta for this node
	dao := servicecontext.GetDAO(ctx).(DAO)

	if err = dao.SetMetadata(request.Node.Uuid, map[string]string{}); err != nil {
		return err
	}

	result.Success = true

	client.Publish(ctx, client.NewPublication(common.TOPIC_META_CHANGES, &tree.NodeChangeEvent{
		Type:   tree.NodeChangeEvent_DELETE,
		Source: request.Node,
	}))

	return nil
}

// Search a stream of nodes based on its metadata
func (s *MetaServer) Search(ctx context.Context, request *tree.SearchRequest, result tree.Searcher_SearchStream) error {

	dao := servicecontext.GetDAO(ctx).(DAO)

	metaByUUID, err := dao.ListMetadata(request.Query.FileName)
	if err != nil {
		return err
	}

	for uuid, metadata := range metaByUUID {
		result.Send(&tree.SearchResponse{
			Node: &tree.Node{
				Uuid:      uuid,
				MetaStore: metadata,
			},
		})
	}

	result.Close()
	return nil
}
