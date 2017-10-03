package cmd

import (
	"context"

	"github.com/micro/cli"
	"github.com/pydio/services/common/log"
	"github.com/pydio/services/scheduler/jobs"
	"go.uber.org/zap"
)

var (
	jobsFlags = []cli.Flag{}

	schedulerJobsCmd = cli.Command{
		Name:   "jobs",
		Usage:  "Start service for CRUD jobs",
		Flags:  jobsFlags,
		Action: mainJobs,
	}
)

func mainJobs(ctx *cli.Context) {

	service, err := jobs.NewSchedulerJobsService(context.Background())
	if err != nil {
		log.Logger(context.Background()).Fatal("Error creating jobs", zap.Error(err))
	}
	if err := service.Run(); err != nil {
		log.Logger(context.Background()).Fatal("Error running jobs", zap.Error(err))
	}

}
