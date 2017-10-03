package cmd

import (
	"context"

	"github.com/micro/cli"
	"github.com/pydio/services/common/log"
	"github.com/pydio/services/data/meta"
	"go.uber.org/zap"
)

var (
	apiMetaFlags = []cli.Flag{}

	apiMetaCmd = cli.Command{
		Name:   "meta",
		Usage:  "Starts gateway API for metadata service.",
		Flags:  apiMetaFlags,
		Action: mainApiMeta,
	}
)

func mainApiMeta(ctx *cli.Context) {

	serv, err := meta.NewMetaApiService(ctx)

	if err != nil {
		log.Logger(context.Background()).Fatal("Error creating meta", zap.Error(err))
	}

	if err := serv.Run(); err != nil {
		log.Logger(context.Background()).Fatal("Error running meta", zap.Error(err))
	}
}
