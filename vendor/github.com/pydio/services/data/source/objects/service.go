package objects

import (
	"fmt"

	//	"os"
	"path/filepath"

	"golang.org/x/net/context"

	micro "github.com/micro/go-micro"
	minio "github.com/pydio/minio-priv/cmd"
	"github.com/pydio/services/common"
	"github.com/pydio/services/common/proto/object"
	"github.com/pydio/services/common/service"
	"github.com/pydio/services/common/service/context"
)

// ObjectHandler definition
type ObjectHandler struct {
	ExternalHost   string
	Port           int
	BucketName     string
	DataSourceName string
}

// StartMinioServer handler
func (o *ObjectHandler) StartMinioServer(dataSourceName string, folderName string, gateway string) {

	params := []string{"minio"}
	if gateway != "" {
		params = append(params, "gateway")
		params = append(params, gateway)
	} else {
		params = append(params, "server")
	}

	configFolder, e := CreateMinioConfigFile(dataSourceName, dataSourceName, dataSourceName+"secret")
	if e != nil {
		//log.Logger(context.TODO()).Fatal("Error creating config", zap.Error(e))
	}
	params = append(params, "--config-dir")
	params = append(params, configFolder)

	if o.Port > 0 {
		params = append(params, "--address")
		params = append(params, fmt.Sprintf(":%d", o.Port))
	}

	if folderName != "" {
		params = append(params, folderName)
	}

	//params = append(params, "--quiet")

	minio.Main(params)

}

// GetHttpURL of handler
func (o *ObjectHandler) GetHttpURL(ctx context.Context, req *object.GetHttpUrlRequest, resp *object.GetHttpUrlResponse) error {

	resp.URL = fmt.Sprintf("%s:%d/%s", o.ExternalHost, o.Port, o.BucketName)
	if o.DataSourceName == "miniods2" {
		resp.Encrypt = true
	}
	return nil

}

// NewObjectsService handler, either with localFolder = path to folder,
// of with gateway and localFolder as bucket name
func NewObjectsService(ctx context.Context, datasource string, folder string, gateway string) (micro.Service, error) {

	srv := service.NewService(
		micro.Name(common.SERVICE_OBJECTS_+datasource),
		micro.Context(ctx),
	)

	engine := &ObjectHandler{
		DataSourceName: datasource,
	}

	srv.Init(micro.AfterStart(func() error {
		ctx = srv.Options().Context
		config := servicecontext.GetConfig(ctx)

		var bucketName string
		var baseFolder string
		if gateway != "" {
			bucketName = folder
			baseFolder = ""
		} else {
			baseFolder, bucketName = filepath.Split(folder)
		}

		engine.ExternalHost = "127.0.0.1"
		engine.BucketName = bucketName

		if port, ok := config.Get("port").(float64); ok {
			engine.Port = int(port)
		}

		if host, ok := config.Get("host").(string); ok {
			engine.ExternalHost = host
		}

		// Start on parent path to serve folder as base bucket
		go engine.StartMinioServer(datasource, baseFolder, gateway)

		object.RegisterS3EndpointHandler(srv.Server(), engine)

		return nil
	}))

	return srv, nil
}
