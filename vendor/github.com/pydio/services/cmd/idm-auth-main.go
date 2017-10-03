package cmd

import (
	"context"

	"go.uber.org/zap"

	"github.com/micro/cli"
	"github.com/pydio/services/common/log"
	"github.com/pydio/services/idm/auth"
)

var (
	authCmd = cli.Command{
		Name:   "auth",
		Usage:  "Starts the OAuth2 authentication service.",
		Action: mainAuth,
	}
)

func mainAuth(ctx *cli.Context) {
	serv, err := auth.NewAuthService(context.Background())
	if err != nil {
		log.Logger(context.Background()).Fatal("Error creating auth", zap.Error(err))
	}

	if err := serv.Run(); err != nil {
		log.Logger(context.Background()).Fatal("Error running auth", zap.Error(err))
	}
}
