package handler

import (
	"go.uber.org/zap"

	"strings"
	"sync"

	"github.com/micro/go-micro/client"
	"github.com/micro/go-micro/errors"
	"github.com/pydio/services/common"
	"github.com/pydio/services/common/event"
	"github.com/pydio/services/common/log"
	protosync "github.com/pydio/services/common/proto/sync"
	"github.com/pydio/services/common/proto/tree"
	"github.com/pydio/services/search/dao"
	"golang.org/x/net/context"
)

type SearchServer struct {
	Engine        dao.SearchEngine
	eventsChannel chan *event.EventWithContext
	TreeClient    tree.NodeProviderClient
}

// CreateNodeChangeSubscriber that will treat events for the meta server
func (s *SearchServer) CreateNodeChangeSubscriber() *EventsSubscriber {

	if s.eventsChannel == nil {
		s.initEventsChannel()
	}
	subscriber := &EventsSubscriber{
		outputChannel: s.eventsChannel,
	}
	return subscriber
}

func (s *SearchServer) initEventsChannel() {

	s.eventsChannel = make(chan *event.EventWithContext)
	go func() {
		for eventWCtx := range s.eventsChannel {
			s.processEvent(eventWCtx.Context, eventWCtx.Event)
		}
	}()
}

func (s *SearchServer) processEvent(ctx context.Context, e *tree.NodeChangeEvent) {

	log.Logger(ctx).Info("processEvent", zap.Any("event", e))

	switch e.GetType() {
	case tree.NodeChangeEvent_CREATE:
		// Let's extract the basic information from the tree and store it
		s.Engine.IndexNode(ctx, e.Target)
		break
	case tree.NodeChangeEvent_UPDATE_PATH:
		// Let's extract the basic information from the tree and store it
		s.Engine.IndexNode(ctx, e.Target)
		break
	case tree.NodeChangeEvent_UPDATE_META:
		// Let's extract the basic information from the tree and store it
		s.Engine.IndexNode(ctx, e.Target)
		break
	case tree.NodeChangeEvent_UPDATE_CONTENT:
		// We may have to store the metadata again
		s.Engine.IndexNode(ctx, e.Target)
		break
	case tree.NodeChangeEvent_DELETE:
		// Lets delete all metadata
		s.Engine.DeleteNode(ctx, e.Source)
	default:
		log.Logger(ctx).Error("Could not recognize event type", zap.Any("type", e.GetType()))
	}
}

func (s *SearchServer) Search(ctx context.Context, req *tree.SearchRequest, streamer tree.Searcher_SearchStream) error {

	resultsChan := make(chan *tree.Node)
	doneChan := make(chan bool)
	defer close(resultsChan)
	defer close(doneChan)

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case node := <-resultsChan:
				if node != nil {

					log.Logger(ctx).Info("Search", zap.String("uuid", node.Uuid))

					if req.Details {
						response, e := s.TreeClient.ReadNode(ctx, &tree.ReadNodeRequest{Node: &tree.Node{
							Uuid: node.Uuid,
						}})
						if e == nil {
							streamer.Send(&tree.SearchResponse{Node: response.Node})
						} else if errors.Parse(e.Error()).Code == 404 {

							log.Logger(ctx).Error("Found node that does not exists, send event to make sure all is sync'ed.", zap.String("uuid", node.Uuid))

							client.Publish(ctx, client.NewPublication(common.TOPIC_TREE_CHANGES, &tree.NodeChangeEvent{
								Type:   tree.NodeChangeEvent_DELETE,
								Source: node,
							}))

						}
					} else {
						log.Logger(ctx).Info("No Details needed, sending back %v", zap.String("uuid", node.Uuid))
						streamer.Send(&tree.SearchResponse{Node: node})
					}

				}
			case <-doneChan:
				return
			}
		}
	}()

	err := s.Engine.SearchNodes(ctx, req.GetQuery(), req.GetFrom(), req.GetSize(), resultsChan, doneChan)
	if err != nil {
		return err
	}
	wg.Wait()
	return nil
}

func (s *SearchServer) TriggerResync(c context.Context, req *protosync.ResyncRequest, resp *protosync.ResyncResponse) error {

	go func() {
		bg := context.Background()
		s.Engine.ClearIndex(bg)

		dsStream, err := s.TreeClient.ListNodes(bg, &tree.ListNodesRequest{
			Node:      &tree.Node{Path: ""},
			Recursive: true,
		})
		if err != nil {
			log.Logger(c).Error("Resync", zap.Error(err))
			return
		}
		defer dsStream.Close()
		for {
			response, e := dsStream.Recv()
			if e != nil || response == nil {
				break
			}
			if response.Node.GetUuid() != "DATA SOURCE" && !strings.HasSuffix(response.Node.GetPath(), ".__pydio") {
				s.Engine.IndexNode(bg, response.Node)
			}
		}

	}()

	resp.Success = true

	return nil
}
