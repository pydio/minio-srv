package cmd

import (
	"context"

	"github.com/micro/cli"
	"github.com/pydio/services/common/log"
	"github.com/pydio/services/data/tree"
	"go.uber.org/zap"
)

var (
	dataTreeFlags = []cli.Flag{}

	dataTreeCmd = cli.Command{
		Name:   "tree",
		Usage:  "Starts the role service.",
		Flags:  dataTreeFlags,
		Action: mainDataTree,
	}
)

func mainDataTree(ctx *cli.Context) {

	serv, err := tree.NewTreeMicroService()

	if err != nil {
		log.Logger(context.Background()).Fatal("Error creating tree", zap.Error(err))
	}

	if err := serv.Run(); err != nil {
		log.Logger(context.Background()).Fatal("Error running tree", zap.Error(err))
	}
}
