package versions

import (
	"golang.org/x/net/context"
	"github.com/micro/go-micro"
	"github.com/pydio/services/common/service"
	"github.com/pydio/services/common"
	"github.com/pydio/services/common/proto/tree"
	"github.com/pydio/services/common/proto/jobs"
	"github.com/pydio/services/common/config"
	"path"
)

func NewVersionMicroService(ctx context.Context) (micro.Service, error) {

	srv := service.NewService(
		micro.Name(common.SERVICE_VERSIONS),
	)

	dataDir, err := config.ApplicationDataDir()
	if err != nil {
		return nil, err
	}
	store, err := NewBoltStore(path.Join(dataDir, "versions.db"))
	engine := &Handler{
		db:store,
	}

	tree.RegisterNodeVersionerHandler(srv.Server(), engine)
	srv.Init(micro.AfterStart(func() error {
		jobsClient := jobs.NewJobServiceClient(common.SERVICE_JOBS, srv.Client())
		for _, j := range getDefaultJobs() {
			jobsClient.PutJob(context.Background(), &jobs.PutJobRequest{Job:j})
		}
		return nil
	}))


	return srv, nil
}
