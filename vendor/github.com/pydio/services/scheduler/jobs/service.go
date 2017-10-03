package jobs

import (
	"context"
	"path"

	micro "github.com/micro/go-micro"
	"github.com/pydio/services/common"
	"github.com/pydio/services/common/config"
	"github.com/pydio/services/common/proto/jobs"
	"github.com/pydio/services/common/service"
	"github.com/pydio/services/common/log"
)

func NewSchedulerJobsService(ctx context.Context) (micro.Service, error) {

	dataDir, err := config.ApplicationDataDir()
	if err != nil {
		return nil, err
	}
	store, err := NewBoltStore(path.Join(dataDir, "jobs.db"))
	srv := service.NewService(
		micro.Name(common.SERVICE_JOBS),
		micro.BeforeStop(func() error {
			log.Logger(ctx).Info("Closing Bolt Store for Jobs service")
			store.Close()
			return nil
		}),
	)
	if err != nil {
		return nil, err
	}
	handler := &JobsHandler{
		store: store,
	}
	jobs.RegisterJobServiceHandler(srv.Server(), handler)

	for _, j := range getDefaultJobs() {
		handler.PutJob(context.Background(), &jobs.PutJobRequest{Job: j}, &jobs.PutJobResponse{})
	}

	return srv, nil
}
