package cmd

import (
	"github.com/pydio/services/common/log"
	"go.uber.org/zap"

	"github.com/micro/cli"
	"github.com/pydio/services/broker/activity"
)

var (
	apiActivityFlags = []cli.Flag{}

	apiActivityCmd = cli.Command{
		Name:   "activity",
		Usage:  "Starts JSON API for activity service.",
		Flags:  apiActivityFlags,
		Action: mainApiActivity,
	}
)

func mainApiActivity(ctx *cli.Context) {

	srv, err := activity.NewActivityApiService(ctx)

	if err != nil {
		log.Logger(srv.Options().Context).Fatal("Failed to init", zap.Error(err))
		return
	}

	if err := srv.Run(); err != nil {
		log.Logger(srv.Options().Context).Fatal("Failed to run", zap.Error(err))
	}
}
