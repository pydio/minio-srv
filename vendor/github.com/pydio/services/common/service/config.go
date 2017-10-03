package service

import (
	"encoding/json"
	"time"

	"github.com/micro/config-srv/db"
	micro "github.com/micro/go-micro"
	"github.com/micro/go-micro/errors"
	"github.com/micro/go-micro/selector"
	"github.com/micro/go-micro/server"
	"github.com/pydio/services/common"
	"github.com/pydio/services/common/config"
	"github.com/pydio/services/common/log"
	"github.com/pydio/services/common/service/context"
	"go.uber.org/zap"

	"github.com/micro/config-srv/proto/config"
	"golang.org/x/net/context"
)

func newConfigProvider(service micro.Service) error {

	name := service.Server().Options().Name

	if name == common.SERVICE_CONFIG {
		return nil
	}

	ctx := service.Options().Context

	var options []micro.Option

	// Going to get the configs from the config service
	options = append(options, micro.BeforeStart(func() error {

		client := go_micro_srv_config_config.NewConfigClient(common.SERVICE_CONFIG, service.Client())

		resp, err := client.Read(ctx, &go_micro_srv_config_config.ReadRequest{
			Id:   "services/" + service.Server().Options().Name,
			Path: "service",
		})

		if err == nil && resp.GetChange() != nil {

			stringData := resp.GetChange().GetChangeSet().Data

			var config config.Map

			json.Unmarshal([]byte(stringData), &config)

			ctx = servicecontext.WithConfig(ctx, config)

			service.Init(micro.Context(ctx))
		} else {

			detailedErr := errors.Parse(err.Error())

			switch detail := detailedErr.Detail; detail {
			case selector.ErrNoneAvailable.Error():
				{
					<-time.After(3 * time.Second)

					log.Logger(ctx).Error("Could not contact config service, trying again in 3s. Did you start config?")
				}
			case db.ErrNotFound.Error():
				{
					return nil
				}
			default:
				{
					log.Logger(ctx).Error("Could not contact load config", zap.String("service", name), zap.Error(err))
				}
			}
		}

		return nil
	}))

	options = append(options, micro.WrapHandler(NewConfigHandlerWrapper(service)))

	service.Init(options...)

	log.Logger(ctx).Info("Service initialized")

	return nil
}

// NewConfigHandlerWrapper wraps the service config within the handler so it can be accessed by the handler itself.
func NewConfigHandlerWrapper(service micro.Service) server.HandlerWrapper {
	return func(h server.HandlerFunc) server.HandlerFunc {
		return func(ctx context.Context, req server.Request, rsp interface{}) error {

			config := servicecontext.GetConfig(service.Options().Context)

			ctx = servicecontext.WithConfig(ctx, config)

			return h(ctx, req, rsp)
		}
	}
}
