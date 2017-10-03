package cmd

import (
	"context"

	"go.uber.org/zap"

	"github.com/micro/cli"
	"github.com/pydio/services/broker/activity"
	"github.com/pydio/services/common/log"
)

var (
	activityFlags = []cli.Flag{}

	activityCmd = cli.Command{
		Name:   "activity",
		Usage:  "Starts activity streams service.",
		Flags:  activityFlags,
		Action: mainActivity,
	}
)

func mainActivity(ctx *cli.Context) {

	serv, err := activity.NewActivityService(context.Background())

	if err != nil {
		log.Logger(context.Background()).Fatal("Error creating activity", zap.Error(err))
	}

	if err := serv.Run(); err != nil {
		log.Logger(context.Background()).Fatal("Error running activity", zap.Error(err))
	}
}
