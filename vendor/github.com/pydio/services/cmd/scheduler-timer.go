package cmd

import (
	"context"

	"github.com/micro/cli"
	"github.com/pydio/services/common/log"
	"github.com/pydio/services/scheduler/timer"
	"go.uber.org/zap"
)

var (
	timerFlags = []cli.Flag{}

	schedulerTimerCmd = cli.Command{
		Name:   "timer",
		Usage:  "Start simple scheduler triggering event on a timely basis",
		Flags:  timerFlags,
		Action: mainTimer,
	}
)

func mainTimer(ctx *cli.Context) {

	service, err := timer.NewSchedulerTimerService(context.Background())
	if err != nil {
		log.Logger(context.Background()).Fatal("Error creating service timer", zap.Error(err))
	}
	if err := service.Run(); err != nil {
		log.Logger(context.Background()).Fatal("Error running service timer", zap.Error(err))
	}

}
