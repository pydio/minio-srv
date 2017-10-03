package cmd

import (
	"context"

	"github.com/micro/cli"
	"github.com/pydio/services/common/log"
	"github.com/pydio/services/data/versions"
	"go.uber.org/zap"
)

var (
	versionFlags = []cli.Flag{}

	dataVersionCmd = cli.Command{
		Name:   "versions",
		Usage:  "Starts the versionning service.",
		Flags:  versionFlags,
		Action: mainVersions,
	}
)

func mainVersions(ctx *cli.Context) {

	serv, err := versions.NewVersionMicroService(context.Background())

	if err != nil {
		log.Logger(context.Background()).Fatal("Error creating versionning service", zap.Error(err))
	}

	if err := serv.Run(); err != nil {
		log.Logger(context.Background()).Fatal("Error running versionning service", zap.Error(err))
	}
}
