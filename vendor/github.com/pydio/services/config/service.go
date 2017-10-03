package config

import (
	"context"
	"github.com/pydio/services/common/log"

	"go.uber.org/zap"

	"github.com/micro/config-srv/config"
	"github.com/micro/config-srv/db"

	proto "github.com/micro/config-srv/proto/config"
	"github.com/micro/go-micro"

	"github.com/micro/config-srv/handler"

	// db

	// Trace
	"github.com/pydio/services/common"
	"github.com/pydio/services/common/service"

	// Registering the config database
	"github.com/micro/config-srv/db/mysql"
)

// NewConfigMicroService definition
func NewConfigMicroService(filename string) micro.Service {

	loader := new(Loader)
	handler := new(handler.Config)

	service := service.NewService(
		micro.Name(common.SERVICE_CONFIG),
		micro.BeforeStart(loader.parser(filename)),
		micro.BeforeStart(func() error {
			url, err := loader.GetConfigServiceDSN()
			if err != nil {
				return err
			}
			mysql.Url = url
			return db.Init()
		}),
		micro.AfterStart(loader.filler(handler)),
	)

	proto.RegisterConfigHandler(service.Server(), handler)

	// subcriber to watches
	service.Server().Subscribe(service.Server().NewSubscriber(config.WatchTopic, config.Watcher))

	if err := config.Init(); err != nil {
		log.Logger(context.TODO()).Fatal("Error initialising service", zap.Error(err))
	}

	return service
}
