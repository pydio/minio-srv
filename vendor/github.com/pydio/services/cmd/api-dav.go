package cmd

import (
	"context"

	"github.com/micro/cli"
	"github.com/pydio/services/client/dav"
	"github.com/pydio/services/common/log"
	"go.uber.org/zap"
)

var (
	apiDavFlags = []cli.Flag{}

	apiDavCmd = cli.Command{
		Name:   "dav",
		Usage:  "Starts WebDAV gateway for tree service.",
		Flags:  apiDavFlags,
		Action: mainApiDav,
	}
)

func mainApiDav(ctx *cli.Context) {

	serv, err := dav.NewDAVService(ctx)

	if err != nil {
		log.Logger(context.Background()).Fatal("Error creating DAV", zap.Error(err))
	}

	if err := serv.Run(); err != nil {
		log.Logger(context.Background()).Fatal("Error running DAV", zap.Error(err))
	}
}
