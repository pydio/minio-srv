package activity

import "github.com/pydio/services/common/proto/activity"

type OwnerType string

type BoxName string

const (
	BoxInbox BoxName = "inbox"
	BoxOutbox BoxName = "outbox"
	BoxSubscriptions BoxName = "subscriptions"
	BoxLastRead BoxName = "lastread"

	OwnerTypeUser OwnerType = "users"
	OwnerTypeNode OwnerType = "nodes"
)

type DAO interface{

	// Post an activity to target inbox
	PostActivity(ownerType OwnerType, ownerId string, boxName BoxName, object *activity.Object) error

	// Update Subscription status
	UpdateSubscription(userId string,  toObjectType OwnerType, toObjectId string, status bool) error

	// List subscriptions on a given object
	// Returns a map of userId => status (true/false, required to disable default subscriptions like workspaces)
	ListSubscriptions(objectType OwnerType, objectIds []string) (userIds map[string]bool, err error)

	// Count the number of unread activities in user "Inbox" box
	CountUnreadForUser(userId string) int

	// Load activities for a given owner. Targets "outbox" by default
	ActivitiesFor(ownerType OwnerType, ownerId string, boxName BoxName, result chan * activity.Object, done chan bool) error

	// Should be wired to "USER_DELETE" and "NODE_DELETE" events
	// to remove (or archive?) deprecated queues
	Delete(ownerType OwnerType, ownerId string) error

}