package cmd

import (
	"context"

	"github.com/micro/cli"
	"github.com/pydio/services/common/log"
	"github.com/pydio/services/scheduler/tasks"
	"go.uber.org/zap"
)

var (
	tasksFlags = []cli.Flag{}

	schedulerTasksCmd = cli.Command{
		Name:   "tasks",
		Usage:  "Actual tasks runner",
		Flags:  tasksFlags,
		Action: mainTasks,
	}
)

func mainTasks(ctx *cli.Context) {

	service, err := tasks.NewSchedulerTasksService(context.Background())
	if err != nil {
		log.Logger(context.Background()).Fatal("Error creating service tasks", zap.Error(err))
	}
	if err := service.Run(); err != nil {
		log.Logger(context.Background()).Fatal("Error running service tasks", zap.Error(err))
	}

}
