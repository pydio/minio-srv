package activity

import (
	"context"
	"path"

	micro "github.com/micro/go-micro"
	"github.com/pydio/services/common"
	"github.com/pydio/services/common/config"
	"github.com/pydio/services/common/proto/activity"
	"github.com/pydio/services/common/proto/tree"
	"github.com/pydio/services/common/service"
)

func NewActivityService(ctx context.Context) (micro.Service, error) {

	srv := service.NewService(
		micro.Name(common.SERVICE_ACTIVITY),
	)

	filePath, err := config.ApplicationDataDir()
	if err != nil {
		return nil, err
	}

	fileName := path.Join(filePath, "activities.db")

	// TODO - should be fitting with the SQL DAO impl
	bolt, _ := NewBoltImpl(fileName, 1000)

	// Register Subscribers
	subscriber := &MicroEventsSubscriber{
		store:  bolt,
		client: tree.NewNodeProviderClient(common.SERVICE_TREE, srv.Client()),
	}
	handler := &Handler{
		db: bolt,
	}
	if err := srv.Server().Subscribe(srv.Server().NewSubscriber(common.TOPIC_TREE_CHANGES, subscriber)); err != nil {
		return nil, err
	}
	/*
		if err := service.Server().Subscribe(service.Server().NewSubscriber(common.TOPIC_META_CHANGES,subscriber)); err != nil {
			return nil, err
		}
	*/

	activity.RegisterActivityServiceHandler(srv.Server(), handler)

	return srv, nil

}
