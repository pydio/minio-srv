package handler

import (
	"github.com/pydio/services/common/event"
	"github.com/pydio/services/common/proto/tree"
	"golang.org/x/net/context"
)

// EventsSubscriber definition
type EventsSubscriber struct {
	outputChannel chan *event.EventWithContext
}

// Handle the events received and send them to the subscriber
func (e *EventsSubscriber) Handle(ctx context.Context, msg *tree.NodeChangeEvent) error {

	e.outputChannel <- &event.EventWithContext{
		Context: ctx,
		Event:   msg,
	}
	return nil
}
