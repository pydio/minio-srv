package cmd

import (
	"context"

	"go.uber.org/zap"

	"github.com/micro/cli"
	"github.com/pydio/services/common/log"
	"github.com/pydio/services/idm/role"
)

var (
	apiRoleFlags = []cli.Flag{}

	apiRoleCmd = cli.Command{
		Name:   "role",
		Usage:  "Starts gateway API for role service.",
		Flags:  apiRoleFlags,
		Action: mainAPIRole,
	}
)

func mainAPIRole(ctx *cli.Context) {

	serv, err := role.NewAPIService(ctx)

	if err != nil {
		log.Logger(context.Background()).Fatal("Error creating role", zap.Error(err))
	}

	if err := serv.Run(); err != nil {
		log.Logger(context.Background()).Fatal("Error running role", zap.Error(err))
	}
}
