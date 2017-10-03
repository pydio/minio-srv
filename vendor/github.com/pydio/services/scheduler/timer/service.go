package timer

import (
	"context"

	micro "github.com/micro/go-micro"
	"github.com/pydio/services/common"
	"github.com/pydio/services/common/proto/jobs"
	"github.com/pydio/services/common/service"
)

func NewSchedulerTimerService(ctx context.Context) (micro.Service, error) {

	srv := service.NewService(
		micro.Name(common.SERVICE_TIMER),
	)

	producer := NewEventProducer(jobs.NewJobServiceClient(common.SERVICE_JOBS, srv.Client()))
	subscriber := &JobsEventsSubscriber{
		Producer: producer,
	}

	srv.Server().Subscribe(
		srv.Server().NewSubscriber(
			common.TOPIC_JOB_CONFIG_EVENT,
			subscriber,
		),
	)

	producer.Start()

	return srv, nil
}
