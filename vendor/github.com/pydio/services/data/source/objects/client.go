package objects

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/micro/go-micro/client"
	"github.com/pydio/services/common/log"
	"github.com/pydio/services/common/proto/object"
)

func GetS3UrlWithRetries(ctx context.Context, name string, client client.Client, count int) string {

	if count > 4 {
		return ""
	}

	var url string

	objectClient := object.NewS3EndpointClient(name, client)
	response, err := objectClient.GetHttpURL(ctx, &object.GetHttpUrlRequest{})
	if err == nil && response.URL != "" {
		log.Logger(ctx).Debug("GetS3UrlWithRetries", zap.String("url", response.URL))
		url = response.URL
	} else {
		log.Logger(ctx).Warn("Could not contact Object service, retrying in 3s...")
		time.Sleep(3 * time.Second)
		url = GetS3UrlWithRetries(ctx, name, client, count+1)
	}

	return url
}
