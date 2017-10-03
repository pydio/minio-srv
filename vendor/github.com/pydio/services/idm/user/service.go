package user

import (
	"context"

	"github.com/micro/go-micro"
	"github.com/pydio/services/common"
	"github.com/pydio/services/common/proto/idm"
	"github.com/pydio/services/common/service"
	"github.com/pydio/services/common/service/context"
)

// NewMicroService for the package
func NewMicroService(ctx context.Context) micro.Service {

	srv := service.NewService(
		micro.Name(common.SERVICE_USER),
		micro.Context(servicecontext.WithDAO(ctx, NewMySQL())),
	)

	idm.RegisterUserServiceHandler(srv.Server(), new(Handler))

	return srv
}
