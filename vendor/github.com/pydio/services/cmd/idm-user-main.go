package cmd

import (
	"context"

	"github.com/micro/cli"
	"github.com/pydio/services/common/log"
	"github.com/pydio/services/idm/user"
	"go.uber.org/zap"
)

var userCmd = cli.Command{
	Name:   "user",
	Usage:  "Starts the user service.",
	Action: mainUser,
}

func mainUser(ctx *cli.Context) {

	service := user.NewMicroService(context.Background())

	if err := service.Run(); err != nil {
		log.Logger(service.Options().Context).Fatal("Error running user", zap.Error(err))
	}
}
