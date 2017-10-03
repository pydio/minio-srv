package cmd

import (
	"context"

	"github.com/micro/cli"
	"github.com/pydio/services/common/log"
	"github.com/pydio/services/idm/workspace"
	"go.uber.org/zap"
)

var (
	apiWorkspaceFlags = []cli.Flag{}

	apiWorkspaceCmd = cli.Command{
		Name:   "workspace",
		Usage:  "Starts gateway API for workspace service.",
		Flags:  apiWorkspaceFlags,
		Action: mainAPIWorkspace,
	}
)

func mainAPIWorkspace(ctx *cli.Context) {

	serv, err := workspace.NewAPIService(ctx)

	if err != nil {
		log.Logger(context.Background()).Fatal("Error creating workspace", zap.Error(err))
	}

	if err := serv.Run(); err != nil {
		log.Logger(context.Background()).Fatal("Error running workspace", zap.Error(err))
	}
}
