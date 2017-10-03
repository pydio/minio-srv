package activity

import (
	"sync"

	"github.com/labstack/gommon/log"
	"github.com/micro/go-micro/errors"
	"github.com/pydio/services/common"
	"github.com/pydio/services/common/proto/activity"
	"golang.org/x/net/context"
)

type Handler struct {
	db DAO
}

func (h *Handler) PostActivity(ctx context.Context, stream activity.ActivityService_PostActivityStream) error {
	return nil
}

func (h *Handler) StreamActivities(ctx context.Context, request *activity.StreamActivitiesRequest, stream activity.ActivityService_StreamActivitiesStream) error {

	result := make(chan *activity.Object)
	done := make(chan bool)

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case ac := <-result:
				stream.Send(&activity.StreamActivitiesResponse{
					Activity: ac,
				})
			case <-done:
				return
			}
		}
	}()

	boxName := BoxOutbox
	if request.BoxName == "inbox" {
		boxName = BoxInbox
	}

	log.Printf("Should get activities for %v %v", request.Context, request.ContextData)
	if request.Context == activity.StreamContext_NODE_ID {

		log.Printf("Should get activities for node %v", request.ContextData)
		h.db.ActivitiesFor(OwnerTypeNode, request.ContextData, boxName, result, done)
		wg.Wait()

	} else if request.Context == activity.StreamContext_USER_ID {

		log.Printf("Should get activities for user %v", request.ContextData)
		h.db.ActivitiesFor(OwnerTypeUser, request.ContextData, boxName, result, done)
		wg.Wait()
	}

	return nil
}

func (h *Handler) Subscribe(ctx context.Context, request *activity.SubscribeRequest, resp *activity.SubscribeResponse) (err error) {

	subscription := request.Subscription

	var ownerType OwnerType
	if subscription.ObjectType == activity.SubscriptionObjectType_NODE {
		ownerType = OwnerTypeNode
	} else if subscription.ObjectType == activity.SubscriptionObjectType_USER {
		ownerType = OwnerTypeUser
	} else {
		return errors.BadRequest(common.SERVICE_ACTIVITY, "Unsupported object type for subscription")
	}

	resp.Subscription = subscription
	return h.db.UpdateSubscription(subscription.UserId, ownerType, subscription.ObjectId, subscription.Status)

}

func (h *Handler) UnreadActivitiesNumber(ctx context.Context, request *activity.UnreadActivitiesRequest, response *activity.UnreadActivitiesResponse) error {

	number := h.db.CountUnreadForUser(request.UserId)
	response.Number = int32(number)

	return nil
}
