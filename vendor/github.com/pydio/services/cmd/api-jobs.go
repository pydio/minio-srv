package cmd

import (
	"github.com/pydio/services/common/log"
	"go.uber.org/zap"

	"github.com/micro/cli"
	"github.com/pydio/services/scheduler/jobs"
)

var (
	apiJobsFlags = []cli.Flag{}

	apiJobsCmd = cli.Command{
		Name:   "jobs",
		Usage:  "Starts JSON API for listing & updating scheduler jobs.",
		Flags:  apiJobsFlags,
		Action: mainApiJobs,
	}
)

func mainApiJobs(ctx *cli.Context) {

	srv, err := jobs.NewJobsApiService(ctx)

	if err != nil {
		log.Logger(srv.Options().Context).Fatal("Failed to init", zap.Error(err))
		return
	}

	if err := srv.Run(); err != nil {
		log.Logger(srv.Options().Context).Fatal("Failed to run", zap.Error(err))
	}
}
