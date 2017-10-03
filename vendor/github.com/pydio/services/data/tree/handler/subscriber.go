package handler

import (
	"github.com/micro/go-micro/client"
	"github.com/pydio/services/common"
	"github.com/pydio/services/common/log"
	"github.com/pydio/services/common/proto/tree"
	"go.uber.org/zap"
	"golang.org/x/net/context"
)

type EventSubscriber struct {
	TreeServer  *TreeServer
	EventClient client.Client
}

// Handle incoming INDEX events and resend them as TREE events
func (s *EventSubscriber) Handle(ctx context.Context, msg *tree.NodeChangeEvent) error {

	// Update Source & Target Nodes
	source, target := msg.Source, msg.Target
	if source != nil {
		var dsSource string
		source.GetMeta(common.META_NAMESPACE_DATASOURCE_NAME, &dsSource)
		s.TreeServer.updateDataSourceNode(source, dsSource)
	}
	if target != nil {
		var dsTarget string
		target.GetMeta(common.META_NAMESPACE_DATASOURCE_NAME, &dsTarget)
		s.TreeServer.updateDataSourceNode(target, dsTarget)
	}

	log.Logger(ctx).Info("Handle", zap.Any("source", source), zap.Any("target", target))

	s.EventClient.Publish(ctx, s.EventClient.NewPublication(common.TOPIC_TREE_CHANGES, msg))

	return nil
}
