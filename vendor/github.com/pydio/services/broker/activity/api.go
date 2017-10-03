package activity

import (
	"github.com/pydio/services/common/log"
	"github.com/pydio/services/common/service"

	"github.com/micro/go-micro"
	api "github.com/micro/micro/api/proto"

	"encoding/json"
	"strconv"
	"strings"

	"github.com/micro/cli"
	"github.com/micro/go-micro/errors"
	"github.com/micro/go-micro/metadata"
	"github.com/micro/protobuf/jsonpb"
	"github.com/pydio/services/common"
	"github.com/pydio/services/common/proto/activity"
	"golang.org/x/net/context"
)

type Activity struct {
	ActivityClient activity.ActivityServiceClient
}

func withFakeApiUser(ctx context.Context, req *api.Request) context.Context {

	var md map[string]string
	var ok bool
	if md, ok = metadata.FromContext(ctx); !ok {
		md = make(map[string]string)
	}
	userId := "activity-api-pydio-user"
	if sUser, dOk := req.Get["fake-user"]; dOk {
		userId = strings.Join(sUser.Values, "")
	}
	md[common.PYDIO_CONTEXT_USER_KEY] = userId
	return metadata.NewContext(ctx, md)

}

func activityBuilder(service micro.Service) interface{} {
	return &Activity{
		ActivityClient: activity.NewActivityServiceClient(common.SERVICE_ACTIVITY, service.Client()),
	}
}

func (s *Activity) Stream(ctx context.Context, req *api.Request, rsp *api.Response) error {

	ctx = withFakeApiUser(ctx, req)

	log.Logger(ctx).Info("Received Activity.Stream API request")

	request := &activity.StreamActivitiesRequest{
		BoxName: "outbox", // outbox by default, could be 'inbox' for user wall
	}
	sContext, dOk := req.Get["context"]
	if dOk && strings.Join(sContext.Values, "") == "node" {
		request.Context = activity.StreamContext_NODE_ID
	} else if dOk && strings.Join(sContext.Values, "") == "user" {
		request.Context = activity.StreamContext_USER_ID
	}
	sContextData, dOk := req.Get["context_data"]
	if dOk {
		request.ContextData = strings.Join(sContextData.Values, "")
	}
	if boxNames, bOk := req.Get["box_name"]; bOk {
		request.BoxName = strings.Join(boxNames.Values, "")
	}

	stream, err := s.ActivityClient.StreamActivities(ctx, request)
	defer stream.Close()
	if err != nil {
		return err
	}
	var activities []*activity.Object
	marshaler := &jsonpb.Marshaler{}
	for {
		resp, rErr := stream.Recv()
		if resp == nil {
			break
		} else if rErr != nil {
			return err
		}
		activities = append(activities, resp.Activity)
	}

	rsp.StatusCode = 200
	rsp.Header = make(map[string]*api.Pair, 1)
	rsp.Header["Content-type"] = &api.Pair{
		Key:    "Content-type",
		Values: []string{"application/json; charset=utf8"},
	}

	collection := Collection(activities)
	rsp.Body, _ = marshaler.MarshalToString(collection)

	return nil
}

func (s *Activity) Subscribe(ctx context.Context, req *api.Request, rsp *api.Response) error {

	nodeId := ""
	if nodeIds, ok := req.Get["node_id"]; ok {
		nodeId = strings.Join(nodeIds.Values, "")
	}
	userId := ""
	if userIds, ok := req.Get["user_id"]; ok {
		userId = strings.Join(userIds.Values, "")
	}
	status := true
	if statuses, ok := req.Get["status"]; ok {
		status, _ = strconv.ParseBool(strings.Join(statuses.Values, ""))
	}
	if len(nodeId) == 0 || len(userId) == 0 {
		return errors.BadRequest(common.SERVICE_ACTIVITY, "Please provide nodeId and userId parameters.")
	}

	subscription := &activity.Subscription{
		Status:     status,
		ObjectType: activity.SubscriptionObjectType_NODE,
		ObjectId:   nodeId,
		UserId:     userId,
	}

	response, err := s.ActivityClient.Subscribe(withFakeApiUser(ctx, req), &activity.SubscribeRequest{
		Subscription: subscription,
	})
	if err != nil {
		return err
	}

	rsp.StatusCode = 200
	rsp.Header = make(map[string]*api.Pair, 1)
	rsp.Header["Content-type"] = &api.Pair{
		Key:    "Content-type",
		Values: []string{"application/json; charset=utf8"},
	}
	data, err := json.Marshal(response.Subscription)
	if err != nil {
		return err
	}
	rsp.Body = string(data)

	return err

}

func (s *Activity) Unread(ctx context.Context, req *api.Request, rsp *api.Response) error {

	userId := ""
	if userIds, ok := req.Get["user_id"]; ok {
		userId = strings.Join(userIds.Values, "")
	}
	if userId == "" {
		return errors.BadRequest(common.SERVICE_ACTIVITY, "Missing user_id parameter")
	}
	response, err := s.ActivityClient.UnreadActivitiesNumber(ctx, &activity.UnreadActivitiesRequest{UserId: userId})
	if err != nil {
		return errors.InternalServerError(common.SERVICE_ACTIVITY, err.Error())
	}

	rsp.StatusCode = 200
	rsp.Header = make(map[string]*api.Pair, 1)
	rsp.Header["Content-type"] = &api.Pair{
		Key:    "Content-type",
		Values: []string{"application/json; charset=utf8"},
	}
	data := make(map[string]interface{})
	data["count"] = response.Number
	jsonData, _ := json.Marshal(data)
	rsp.Body = string(jsonData)
	return nil
}

func NewActivityApiService(ctx *cli.Context) (micro.Service, error) {

	srv := service.NewAPIService(activityBuilder, micro.Name(common.SERVICE_API_NAMESPACE_+"activity"))

	return srv, nil
}
