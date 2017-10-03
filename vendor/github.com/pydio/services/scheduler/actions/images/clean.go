package images

import (
	"fmt"
	"github.com/micro/go-micro/client"
	"github.com/micro/go-micro/metadata"
	"github.com/pydio/services/common/log"
	"github.com/pydio/services/common/proto/jobs"
	"github.com/pydio/services/common/views"
	"go.uber.org/zap"
	"golang.org/x/net/context"
	"strings"
	"github.com/pydio/services/common"
)

var (
	cleanThumbTaskName = "actions.images.clean"
)

type CleanThumbsTask struct{
	Client client.Client
}

func (c *CleanThumbsTask) GetName() string {
	return cleanThumbTaskName
}

func (c *CleanThumbsTask) Init(job *jobs.Job, cl client.Client, action *jobs.Action) error {
	c.Client = cl
	return nil
}

func (c *CleanThumbsTask) Run(ctx context.Context, input jobs.ActionMessage) (jobs.ActionMessage, error) {

	if len(input.Nodes) == 0 {
		return input.WithIgnore(), nil
	}

	thumbsClient, thumbsBucket, e := views.GetGenericStoreClient(ctx, common.PYDIO_THUMBSTORE_NAMESPACE, c.Client)
	if meta, mOk := metadata.FromContext(ctx); mOk {
		thumbsClient.PrepareMetadata(map[string]string{
			"x-pydio-user": meta["x-pydio-user"],
		})
		defer thumbsClient.ClearMetadata()
	}
	if e != nil {
		log.Logger(ctx).Debug("Cannot get ThumbStoreClient", zap.Error(e), zap.Any("context", ctx))
		return input.WithError(e), e
	}
	nodeUuid := input.Nodes[0].Uuid
	// List all thumbs starting with node Uuid
	listRes, err := thumbsClient.ListObjects(thumbsBucket, nodeUuid+"-", "", "", 0)
	if err != nil {
		log.Logger(ctx).Debug("Cannot get ThumbStoreClient", zap.Error(err), zap.Any("context", ctx))
		return input.WithError(err), err
	}
	logs := []string{"Removing thumbs associated to node " + nodeUuid}
	for _, oi := range listRes.Contents {
		err := thumbsClient.RemoveObject(thumbsBucket, oi.Key)
		if err != nil {
			log.Logger(ctx).Debug("Cannot get ThumbStoreClient", zap.Error(err))
			return input.WithError(err), err
		}
		logs = append(logs, fmt.Sprintf("Succesfully removed object %s", oi.Key))
	}
	output := jobs.ActionMessage{}
	output.AppendOutput(&jobs.ActionOutput{
		StringBody: strings.Join(logs, "\n"),
	})
	log.Logger(ctx).Debug("Thumbs Clean Output", zap.Any("logs", logs))
	return output, nil
}
