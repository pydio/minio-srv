package websocket

import (
	micro "github.com/micro/go-micro"
	"github.com/pydio/services/common"
	"github.com/pydio/services/common/proto/tree"
	"github.com/pydio/services/common/service"
	"github.com/pydio/services/common/views"
)

func NewWebSocketService(port int) (micro.Service, error) {

	srv := service.NewService(
		micro.Name(common.SERVICE_API_NAMESPACE_ + "websocket"),
	)

	ws := NewWebSocketHandler(port)
	ws.TreeClient = tree.NewNodeProviderClient(common.SERVICE_TREE, srv.Client())
	ws.Router = views.NewStandardRouter(false, true)
	go ws.Run()

	// Register Subscribers
	subscriber := &MicroEventsSubscriber{
		WebSocketHandler: ws,
	}
	if err := srv.Server().Subscribe(srv.Server().NewSubscriber(common.TOPIC_TREE_CHANGES, subscriber)); err != nil {
		return nil, err
	}
	if err := srv.Server().Subscribe(srv.Server().NewSubscriber(common.TOPIC_META_CHANGES, subscriber)); err != nil {
		return nil, err
	}

	return srv, nil

}
