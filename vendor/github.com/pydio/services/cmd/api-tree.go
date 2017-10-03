package cmd

import (
	"context"

	"github.com/micro/cli"
	"github.com/pydio/services/common/log"
	"github.com/pydio/services/data/tree"
	"go.uber.org/zap"
)

var (
	apiTreeFlags = []cli.Flag{}

	apiTreeCmd = cli.Command{
		Name:   "tree",
		Usage:  "Starts gateway API for Tree service.",
		Flags:  apiTreeFlags,
		Action: mainApiTree,
	}
)

func mainApiTree(ctx *cli.Context) {

	serv, err := tree.NewTreeApiService(ctx)

	if err != nil {
		log.Logger(context.Background()).Fatal("Error creating tree", zap.Error(err))
	}

	if err := serv.Run(); err != nil {
		log.Logger(context.Background()).Fatal("Error running tree", zap.Error(err))
	}
}
