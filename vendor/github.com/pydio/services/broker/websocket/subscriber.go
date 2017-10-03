package websocket

import (
	"golang.org/x/net/context"
	"github.com/pydio/services/common/proto/tree"
)

type MicroEventsSubscriber struct{
	WebSocketHandler *WebsocketHandler
}

// Handle the events received and send them to the subscriber
func (e *MicroEventsSubscriber) Handle(ctx context.Context, msg *tree.NodeChangeEvent) error {

	e.WebSocketHandler.BroadcastEvent(ctx, msg)

	return nil

}