package cmd

import (
	"context"

	"github.com/micro/cli"
	"github.com/pydio/services/common/log"
	"github.com/pydio/services/config"
	"go.uber.org/zap"
)

var (
	apiConfigFlags = []cli.Flag{}

	apiConfigCmd = cli.Command{
		Name:   "config",
		Usage:  "Starts gateway API for config service.",
		Flags:  apiConfigFlags,
		Action: mainAPIConfig,
	}
)

func mainAPIConfig(ctx *cli.Context) {

	serv, err := config.NewAPIService(ctx)

	if err != nil {
		log.Logger(context.Background()).Fatal("Error creating Config", zap.Error(err))
	}

	if err := serv.Run(); err != nil {
		log.Logger(context.Background()).Fatal("Error running Config", zap.Error(err))
	}
}
