package activity

import (
	"time"

	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/pydio/services/common/proto/activity"
	"github.com/pydio/services/common/proto/tree"
)

func createObject() *activity.Object {
	return &activity.Object{
		JsonLdContext: "https://www.w3.org/ns/activitystreams",
	}
}

func DocumentActivity(author string, event *tree.NodeChangeEvent) (ac *activity.Object, detectedNode *tree.Node) {

	ac = createObject()
	ac.Name = "File Event"
	switch event.Type {
	case tree.NodeChangeEvent_CREATE:
		// log.Printf("CREATE %v", event.Type)
		ac.Type = activity.ObjectType_Create
		ac.Object = &activity.Object{
			Type: activity.ObjectType_Document,
			Name: event.Target.Path,
			Id:   event.Target.Uuid,
		}
		detectedNode = event.Target
		break
	case tree.NodeChangeEvent_DELETE:
		// log.Printf("DELETE %v", event.Type)
		ac.Type = activity.ObjectType_Delete
		ac.Object = &activity.Object{
			Type: activity.ObjectType_Document,
			Name: event.Source.Path,
			Id:   event.Source.Uuid,
		}
		detectedNode = event.Source
		break
	case tree.NodeChangeEvent_UPDATE_PATH:
		// log.Printf("MOVE %v", event.Type)
		ac.Type = activity.ObjectType_Move
		ac.Object = &activity.Object{
			Type: activity.ObjectType_Document,
			Name: event.Target.Path,
			Id:   event.Target.Uuid,
		}
		ac.Origin = &activity.Object{
			Type: activity.ObjectType_Document,
			Name: event.Source.Path,
			Id:   event.Source.Uuid,
		}
		ac.Target = &activity.Object{
			Type: activity.ObjectType_Document,
			Name: event.Target.Path,
			Id:   event.Target.Uuid,
		}
		detectedNode = event.Target
		break
	case tree.NodeChangeEvent_UPDATE_CONTENT, tree.NodeChangeEvent_UPDATE_META:
		// log.Printf("UPDATE %v", event.Type)
		ac.Type = activity.ObjectType_Update
		ac.Object = &activity.Object{
			Type: activity.ObjectType_Document,
			Name: event.Target.Path,
			Id:   event.Target.Uuid,
		}
		detectedNode = event.Target
		break
	}

	ac.Actor = &activity.Object{
		Type: activity.ObjectType_Person,
		Name: author,
		Id:   author,
	}

	ac.Updated = &timestamp.Timestamp{
		Seconds: time.Now().Unix(),
	}

	return ac, detectedNode

}

func Collection(items []*activity.Object) (c *activity.Object) {

	c = createObject()
	c.Type = activity.ObjectType_Collection
	c.Items = items
	c.TotalItems = int32(len(items))

	return c

}
