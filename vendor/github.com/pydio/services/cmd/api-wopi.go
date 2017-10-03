package cmd

import (
	"context"

	"github.com/micro/cli"
	"github.com/pydio/services/client/wopi"
	"github.com/pydio/services/common/log"
	"go.uber.org/zap"
)

var (
	apiWopiFlags = []cli.Flag{
		cli.IntFlag{
			Name:  "wopi_port",
			Usage: "Port for Wopi server (5014 by default).",
			Value: 5014,
		},
	}

	apiWopiCmd = cli.Command{
		Name:   "wopi",
		Usage:  "Starts Wopi gateway for online collaboration editors.",
		Flags:  apiWopiFlags,
		Action: mainApiWopi,
	}
)

func mainApiWopi(ctx *cli.Context) {

	serv, err := wopi.NewWOPIService(context.Background(), ctx.Int("wopi_port"))

	if err != nil {
		log.Logger(context.Background()).Fatal("Error creating Wopi Service", zap.Error(err))
	}

	if err := serv.Run(); err != nil {
		log.Logger(context.Background()).Fatal("Error running Wopi Service", zap.Error(err))
	}
}
