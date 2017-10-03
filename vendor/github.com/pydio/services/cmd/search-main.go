package cmd

import (
	"context"

	"github.com/micro/cli"
	"github.com/pydio/services/common/log"
	"github.com/pydio/services/search"
	"go.uber.org/zap"
)

var (
	searchCmd = cli.Command{
		Name:   "search",
		Usage:  "Starts a search engine.",
		Flags:  []cli.Flag{},
		Action: mainSearch,
	}
)

func mainSearch(ctx *cli.Context) {

	service, err := search.NewSearchMicroService(context.Background())
	if err != nil {
		log.Logger(service.Options().Context).Fatal("Error creating search", zap.Error(err))
	}

	if err := service.Run(); err != nil {
		log.Logger(service.Options().Context).Fatal("Error running search", zap.Error(err))
	}

}
