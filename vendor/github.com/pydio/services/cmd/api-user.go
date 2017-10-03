package cmd

import (
	"context"

	"github.com/micro/cli"
	"github.com/pydio/services/common/log"
	"github.com/pydio/services/idm/user"
	"go.uber.org/zap"
)

var (
	apiUserFlags = []cli.Flag{}

	apiUserCmd = cli.Command{
		Name:   "user",
		Usage:  "Starts gateway API for user service.",
		Flags:  apiUserFlags,
		Action: mainAPIUser,
	}
)

func mainAPIUser(ctx *cli.Context) {

	serv, err := user.NewAPIService(ctx)

	if err != nil {
		log.Logger(context.Background()).Fatal("Error creating user", zap.Error(err))
	}

	if err := serv.Run(); err != nil {
		log.Logger(context.Background()).Fatal("Error running user", zap.Error(err))
	}
}
