package cmd

import (
	"context"

	"github.com/micro/cli"
	"github.com/pydio/services/common/log"
	"github.com/pydio/services/idm/role"
	"go.uber.org/zap"
)

var (
	roleCmd = cli.Command{
		Name:   "role",
		Usage:  "Starts the role service.",
		Action: mainRole,
	}
)

func mainRole(ctx *cli.Context) {
	serv := role.NewMicroService(context.Background())

	if err := serv.Run(); err != nil {
		log.Logger(context.Background()).Fatal("Error running role", zap.Error(err))
	}
}
