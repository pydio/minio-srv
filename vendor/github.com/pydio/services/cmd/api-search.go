package cmd

import (
	"context"

	"github.com/micro/cli"
	"github.com/pydio/services/common/log"
	"github.com/pydio/services/search"
	"go.uber.org/zap"
)

var (
	apiSearchFlags = []cli.Flag{}

	apiSearchCmd = cli.Command{
		Name:   "search",
		Usage:  "Starts gateway API for search service.",
		Flags:  apiSearchFlags,
		Action: mainApiSearch,
	}
)

func mainApiSearch(ctx *cli.Context) {

	serv, err := search.NewSearchApiService(ctx)

	if err != nil {
		log.Logger(context.Background()).Fatal("Error creating search", zap.Error(err))
	}

	if err := serv.Run(); err != nil {
		log.Logger(context.Background()).Fatal("Error running search", zap.Error(err))
	}
}
