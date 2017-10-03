package tasks

import (
	"context"

	micro "github.com/micro/go-micro"
	"github.com/pydio/services/common"
	"github.com/pydio/services/common/service"

	_ "github.com/pydio/services/scheduler/actions"
	_ "github.com/pydio/services/scheduler/actions/cmd"
	_ "github.com/pydio/services/scheduler/actions/images"
	_ "github.com/pydio/services/scheduler/actions/tree"
	_ "github.com/pydio/services/scheduler/actions/cmd"
	_ "github.com/pydio/services/scheduler/actions/archive"
)

func NewSchedulerTasksService(ctx context.Context) (micro.Service, error) {

	srv := service.NewService(
		micro.Name(common.SERVICE_TASKS),
	)

	multiplexer := NewSubscriber(srv.Client(), srv.Server())
	multiplexer.Init()

	return srv, nil
}
