package meta

import (
	"context"

	micro "github.com/micro/go-micro"
	"github.com/pydio/services/common"
	pydio "github.com/pydio/services/common/proto/tree"
	"github.com/pydio/services/common/service"
	"github.com/pydio/services/common/service/context"
)

func NewMetaMicroService(ctx context.Context) (micro.Service, error) {

	srv := service.NewService(
		micro.Name(common.SERVICE_META),
		micro.Context(servicecontext.WithDAO(ctx, NewMySQL())),
	)

	engine := &MetaServer{}

	pydio.RegisterNodeProviderHandler(srv.Server(), engine)
	pydio.RegisterNodeProviderStreamerHandler(srv.Server(), engine)
	pydio.RegisterNodeReceiverHandler(srv.Server(), engine)
	pydio.RegisterSearcherHandler(srv.Server(), engine)

	// Register Subscribers
	if err := srv.Server().Subscribe(
		srv.Server().NewSubscriber(
			common.TOPIC_TREE_CHANGES,
			engine.CreateNodeChangeSubscriber(),
		),
	); err != nil {
		return nil, err
	}

	return srv, nil
}
