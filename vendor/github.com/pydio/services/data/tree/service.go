package tree

import (
	"strings"

	"go.uber.org/zap"

	micro "github.com/micro/go-micro"
	"github.com/micro/go-micro/registry"
	"github.com/pydio/services/common"
	"github.com/pydio/services/common/log"
	"github.com/pydio/services/common/proto/tree"
	"github.com/pydio/services/common/service"
	"github.com/pydio/services/data/tree/handler"

	"sync"

	"context"

	"github.com/pydio/services/common/proto/object"
	//	"time"
	"github.com/micro/go-micro/server"
)

var (
	srv micro.Service
)

func NewTreeMicroService() (micro.Service, error) {

	srv = service.NewService(
		micro.Name(common.SERVICE_TREE),
	)

	ctx := srv.Options().Context

	dataSources := map[string]handler.DataSource{}
	metaServiceClient := tree.NewNodeProviderClient(common.SERVICE_META, srv.Client())
	metaServiceStreamer := tree.NewNodeProviderStreamerClient(common.SERVICE_META, srv.Client())

	treeServer := &handler.TreeServer{
		ConfigsMutex:        &sync.Mutex{},
		DataSources:         dataSources,
		MetaServiceClient:   metaServiceClient,
		MetaServiceStreamer: metaServiceStreamer,
	}

	eventSubscriber := &handler.EventSubscriber{
		TreeServer:  treeServer,
		EventClient: srv.Client(),
	}

	updateServicesList(ctx, treeServer)

	tree.RegisterNodeProviderHandler(srv.Server(), treeServer)
	tree.RegisterNodeReceiverHandler(srv.Server(), treeServer)

	go watchRegistry(ctx, treeServer)

	// Register Subscribers
	if err := srv.Server().Subscribe(
		srv.Server().NewSubscriber(
			common.TOPIC_INDEX_CHANGES,
			eventSubscriber,
			func(o *server.SubscriberOptions) {
				o.Queue = "tree"
			},
		),
	); err != nil {
		return nil, err
	}

	return srv, nil

}

func updateServicesList(ctx context.Context, treeServer *handler.TreeServer) {

	log.Logger(ctx).Info("Updating Clients List")

	otherServices, _ := srv.Client().Options().Registry.ListServices()
	indexServices := filterServices(otherServices, func(v string) bool {
		return strings.Contains(v, common.SERVICE_INDEX_)
	})
	dataSources := make(map[string]handler.DataSource, len(indexServices))

	log.Logger(ctx).Info("Adding indexServices")

	for _, indexService := range indexServices {
		dataSourceName := strings.TrimLeft(indexService, common.SERVICE_INDEX_)

		writeClient := tree.NewNodeReceiverClient(indexService, srv.Client())
		readClient := tree.NewNodeProviderClient(indexService, srv.Client())
		ds := handler.DataSource{
			Reader: readClient,
			Writer: writeClient,
			S3URL:  "",
		}
		s3Client := object.NewS3EndpointClient(common.SERVICE_OBJECTS_+dataSourceName, srv.Client())
		response, err := s3Client.GetHttpURL(context.Background(), &object.GetHttpUrlRequest{})
		if err == nil {
			ds.S3URL = response.URL
		}

		log.Logger(ctx).Debug("S3 URL", zap.String("url", ds.S3URL))
		dataSources[dataSourceName] = ds
	}
	treeServer.ConfigsMutex.Lock()
	treeServer.DataSources = dataSources
	treeServer.ConfigsMutex.Unlock()

}

func filterServices(vs []*registry.Service, f func(string) bool) []string {
	vsf := make([]string, 0)
	for _, v := range vs {
		if f(v.Name) {
			vsf = append(vsf, v.Name)
		}
	}
	return vsf
}

func watchRegistry(ctx context.Context, treeServer *handler.TreeServer) {

	watcher, err := srv.Client().Options().Registry.Watch()
	if err != nil {
		return
	}
	for {
		result, err := watcher.Next()
		if result != nil && err == nil {
			srv := result.Service
			if strings.Contains(srv.Name, common.SERVICE_INDEX_) && result.Action != "update" {
				updateServicesList(ctx, treeServer)
			}
		}
	}

}
