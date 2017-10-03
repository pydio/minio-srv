package service

import (
	"go.uber.org/zap"

	micro "github.com/micro/go-micro"
	"github.com/micro/go-micro/server"
	"github.com/pydio/services/common/log"
	"github.com/pydio/services/common/service/context"
	"golang.org/x/net/context"
)

func newLogProvider(service micro.Service) error {

	var options []micro.Option
	ctx := service.Options().Context

	name := servicecontext.GetServiceName(ctx)
	color := servicecontext.GetServiceColor(ctx)

	options = append(options, micro.WrapHandler(NewLogHandlerWrapper(name, color)))

	service.Init(options...)

	return nil
}

// NewLogHandlerWrapper wraps a db connection within the handler so it can be accessed by the handler itself.
func NewLogHandlerWrapper(name string, color uint64) server.HandlerWrapper {
	return func(h server.HandlerFunc) server.HandlerFunc {
		return func(ctx context.Context, req server.Request, rsp interface{}) error {
			ctx = servicecontext.WithServiceName(ctx, name)
			ctx = servicecontext.WithServiceColor(ctx, color)

			log.Logger(ctx).Debug("START", zap.Any("req", req))

			err := h(ctx, req, rsp)

			log.Logger(ctx).Debug("END", zap.Any("rsp", rsp))

			return err
		}
	}
}
