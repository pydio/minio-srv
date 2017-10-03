package cmd

import (
	"context"

	"github.com/micro/cli"
	"github.com/pydio/services/common/log"
	"github.com/pydio/services/data/meta"
	"go.uber.org/zap"
)

var (
	metaFlags = []cli.Flag{}

	dataMetaCmd = cli.Command{
		Name:   "meta",
		Usage:  "Starts the meta service.",
		Flags:  metaFlags,
		Action: mainMeta,
	}
)

func mainMeta(ctx *cli.Context) {

	serv, err := meta.NewMetaMicroService(context.Background())

	if err != nil {
		log.Logger(context.Background()).Fatal("Error creating meta", zap.Error(err))
	}

	if err := serv.Run(); err != nil {
		log.Logger(context.Background()).Fatal("Error running meta", zap.Error(err))
	}
}
