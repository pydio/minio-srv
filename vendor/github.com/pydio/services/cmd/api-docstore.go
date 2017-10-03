package cmd

import (
	"github.com/pydio/services/common/log"
	"go.uber.org/zap"

	"github.com/micro/cli"
	"github.com/pydio/services/data/docstore"
)

var (
	apiDocStoreFlags = []cli.Flag{}

	apiDocStoreCmd = cli.Command{
		Name:   "docstore",
		Usage:  "Starts JSON API for CRUD'ing generic JSON or binary document.",
		Flags:  apiDocStoreFlags,
		Action: mainApiDocStore,
	}
)

func mainApiDocStore(ctx *cli.Context) {

	srv, err := docstore.NewDocStoreApiService(ctx)

	if err != nil {
		log.Logger(srv.Options().Context).Fatal("Failed to init", zap.Error(err))
		return
	}

	if err := srv.Run(); err != nil {
		log.Logger(srv.Options().Context).Fatal("Failed to run", zap.Error(err))
	}
}
