package cmd

import (
	"context"

	"github.com/micro/cli"
	"github.com/pydio/services/common/log"
	"github.com/pydio/services/data/docstore"
	"go.uber.org/zap"
)

var (
	docStoreFlags = []cli.Flag{}

	dataDocStoreCmd = cli.Command{
		Name:   "docstore",
		Usage:  "Starts the generic document store.",
		Flags:  docStoreFlags,
		Action: mainDocStore,
	}
)

func mainDocStore(ctx *cli.Context) {

	serv, err := docstore.NewDocumentStoreService(ctx)

	if err != nil {
		log.Logger(context.Background()).Fatal("Error creating docstore", zap.Error(err))
	}

	if err := serv.Run(); err != nil {
		log.Logger(context.Background()).Fatal("Error running docstore", zap.Error(err))
	}
}
