package service

import (
	"time"

	"github.com/micro/go-grpc"
	"github.com/micro/go-micro"
	"github.com/pydio/services/common/service/context"
)

// NewService template
func NewService(opts ...micro.Option) micro.Service {

	service := grpc.NewService(opts...)

	ctx := service.Options().Context
	ctx = servicecontext.WithServiceName(ctx, service.Server().Options().Name)
	ctx = servicecontext.WithServiceColor(ctx)

	// context is always added last - so that there is no override
	service.Init(micro.Context(ctx))

	// newTracer(name, &options)
	newBackoffer(service)
	newConfigProvider(service)
	newDBProvider(service)
	newLogProvider(service)

	// create options with priority for our opts
	options := []micro.Option{
		micro.RegisterTTL(time.Second * 30),
		micro.RegisterInterval(time.Second * 10),
	}

	options = append(options, opts...)

	// context is always added last - so that there is no override
	options = append(options, micro.Context(ctx))

	service.Init(options...)

	return service
}

type ApiHandlerBuilder func(service micro.Service) interface{}

// NewPydioApiService default Constructor. Does not allow anonymous connection, JWT is required
func NewAPIService(builder ApiHandlerBuilder, opts ...micro.Option) micro.Service {

	var options []micro.Option

	service := NewService(opts...)

	newJWTProvider(service)

	service.Init(options...)

	handler := builder(service)
	service.Server().Handle(
		service.Server().NewHandler(handler),
	)

	return service
}

// NewPydioApiService default Constructor. Does not allow anonymous connection, JWT is required
func NewAnonAPIService(builder ApiHandlerBuilder, opts ...micro.Option) micro.Service {

	var options []micro.Option

	service := NewService(opts...)

	newJWTProvider(service)

	service.Init(options...)

	return service

	/*handler := builder(s.service)
	s.service.Server().Handle(
		s.service.Server().NewHandler(handler),
	)*/
}
