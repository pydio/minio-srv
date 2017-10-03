package views

import (
	"encoding/json"
	"github.com/pydio/services/common/log"
	"path/filepath"

	"github.com/micro/config-srv/proto/config"
	"github.com/micro/go-micro/errors"
	"github.com/pydio/minio-go"
	"github.com/pydio/services/common"
	"github.com/pydio/services/common/proto/object"
	"go.uber.org/zap"
	"golang.org/x/net/context"
	"github.com/micro/go-micro/client"
)

func GetGenericStoreClient(ctx context.Context, storeNamespace string, microClient client.Client) (client *minio.Core, bucket string, e error) {

	var dataSource string
	var err error
	dataSource, bucket, err = GetGenericStoreClientConfig(ctx, storeNamespace, microClient)

	s3endpointClient := object.NewS3EndpointClient(common.SERVICE_OBJECTS_ + dataSource, microClient)
	response, err := s3endpointClient.GetHttpURL(ctx, &object.GetHttpUrlRequest{})

	if err != nil {
		return nil, "", err
	}

	url, _ := filepath.Split(response.URL)
	apiKey := dataSource
	apiSecret := dataSource + "secret"
	log.Logger(ctx).Debug("Client", zap.String("url", url), zap.String("apiKey", apiKey), zap.String("apiSecret", apiSecret), zap.String("bucket", bucket))
	client, err = minio.NewCore(url, apiKey, apiSecret, false)
	return client, bucket, err

}

func GetGenericStoreClientConfig(ctx context.Context, storeNamespace string, microClient client.Client) (dataSource string, bucket string, e error) {

	// TMP - TO BE FIXED
	var configKey string
	switch storeNamespace {
	case common.PYDIO_DOCSTORE_BINARIES_NAMESPACE:
		configKey = "services/pydio.docstore-binaries"
		break
	case common.PYDIO_THUMBSTORE_NAMESPACE:
		configKey = "services/pydio.thumbs_store"
		break
	default:
		configKey = "services/pydio." + storeNamespace
		break
	}

	configClient := go_micro_srv_config_config.NewConfigClient(common.SERVICE_CONFIG, microClient)
	resp, err := configClient.Read(ctx, &go_micro_srv_config_config.ReadRequest{
		Id:   configKey,
		Path: "service",
	})
	if err != nil {
		return "", "", err
	}
	if resp.GetChange() == nil {
		return "", "", errors.InternalServerError(storeNamespace, "Cannot find Client configuration for " + storeNamespace)
	}

	stringData := resp.GetChange().GetChangeSet().Data

	config := make(map[string]interface{})
	json.Unmarshal([]byte(stringData), &config)
	ds := config["datasource"].(string)
	bucket = config["bucket"].(string)

	return ds, bucket, nil


}
