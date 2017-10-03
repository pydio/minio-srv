package search

import (
	"context"

	"github.com/pydio/services/common"
	"github.com/pydio/services/common/config"
	"github.com/pydio/services/common/proto/tree"
	"github.com/pydio/services/common/service"
	"github.com/pydio/services/search/handler"

	micro "github.com/micro/go-micro"
	"github.com/pydio/services/common/proto/sync"
	"github.com/pydio/services/search/dao/bleve"
)

func NewSearchMicroService(ctx context.Context) (micro.Service, error) {

	srv := service.NewService(
		micro.Name(common.SERVICE_SEARCH),
		micro.Context(ctx),
	)

	var indexContent bool

	var config config.Map //TODO
	if indexConf := config.Get("indexContent"); indexConf != nil {
		indexContent = config.Get("indexContent").(bool)
	}

	bleveEngine, err := bleve.NewBleveEngine(indexContent)
	if err != nil {
		return nil, err
	}
	server := &handler.SearchServer{
		Engine:     bleveEngine,
		TreeClient: tree.NewNodeProviderClient(common.SERVICE_TREE, srv.Client()),
	}

	tree.RegisterSearcherHandler(srv.Server(), server)
	sync.RegisterSyncEndpointHandler(srv.Server(), server)

	// Register Subscribers
	if err := srv.Server().Subscribe(
		srv.Server().NewSubscriber(
			common.TOPIC_META_CHANGES,
			server.CreateNodeChangeSubscriber(),
		),
	); err != nil {
		return nil, err
	}

	return srv, err
}
