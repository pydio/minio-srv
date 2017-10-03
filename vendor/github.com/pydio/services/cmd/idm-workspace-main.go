package cmd

import (
	"context"

	"github.com/micro/cli"
	"github.com/pydio/services/common/log"
	"github.com/pydio/services/idm/workspace"
	"go.uber.org/zap"
)

var (
	workspaceCmd = cli.Command{
		Name:   "workspace",
		Usage:  "Starts the workspace service.",
		Action: mainWorkspace,
	}
)

func mainWorkspace(ctx *cli.Context) {
	service := workspace.NewMicroService(context.Background())

	if err := service.Run(); err != nil {
		log.Logger(service.Options().Context).Fatal("Error running workspace", zap.Error(err))
	}
}
