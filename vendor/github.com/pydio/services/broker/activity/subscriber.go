package activity

import (
	"github.com/pydio/services/common/log"
	"go.uber.org/zap"

	"github.com/micro/go-micro/metadata"
	"github.com/pydio/services/common"
	"github.com/pydio/services/common/proto/tree"
	"golang.org/x/net/context"
)

type MicroEventsSubscriber struct {
	store  DAO
	client tree.NodeProviderClient
}

// Handle the events received and send them to the subscriber
func (e *MicroEventsSubscriber) Handle(ctx context.Context, msg *tree.NodeChangeEvent) error {

	author := "anonymous"
	meta, ok := metadata.FromContext(ctx)
	if ok {
		user, exists := meta[common.PYDIO_CONTEXT_USER_KEY]
		if exists {
			log.Logger(ctx).Info("Handle", zap.String("user", user))
			author = user
		}
	}
	log.Logger(ctx).Info("Fan out event to activities", zap.Any("message", msg))
	// Create Activities and post them to associated inboxes
	ac, Node := DocumentActivity(author, msg)
	if Node != nil && Node.Uuid != "" {

		//
		// Post to the initial node Outbox
		//
		log.Logger(ctx).Info("Posting Activity to node outbox")
		e.store.PostActivity(OwnerTypeNode, Node.Uuid, BoxOutbox, ac)

		//
		// Post to the author Outbox
		//
		log.Logger(ctx).Info("Posting Activity to author outbox")
		e.store.PostActivity(OwnerTypeUser, author, BoxOutbox, ac)

		//
		// Post to parents Outbox'es as well
		//
		log.Logger(ctx).Info("Listing Parent nodes to post activities")
		streamer, err := e.client.ListNodes(ctx, &tree.ListNodesRequest{
			Node:      Node,
			Ancestors: true,
		})
		parentUuids := []string{Node.Uuid}
		if err != nil {
			return err
		}

		for {
			listResp, err := streamer.Recv()
			if listResp == nil || err != nil {
				break
			}
			uuid := listResp.Node.Uuid
			path := listResp.Node.Path

			streamer.Close()

			parentUuids = append(parentUuids, uuid)
			go func() {
				log.Logger(ctx).Info("Posting activity to parent node", zap.String("path", path))
				e.store.PostActivity(OwnerTypeNode, uuid, BoxOutbox, ac)
			}()
		}

		//
		// Find followers and post activity to their Inbox
		//
		followingUsers, err := e.store.ListSubscriptions(OwnerTypeNode, parentUuids)
		log.Logger(ctx).Info("Listing followers on node and its parents", zap.Any("users", followingUsers))
		if err != nil {
			return err
		}
		for followerId, followStatus := range followingUsers {

			if followStatus == false {
				continue
			}

			go e.store.PostActivity(OwnerTypeUser, followerId, BoxInbox, ac)

		}

	}

	return nil

}
