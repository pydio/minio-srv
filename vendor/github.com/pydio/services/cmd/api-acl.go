package cmd

import (
	"context"

	"github.com/micro/cli"
	"github.com/pydio/services/common/log"
	"github.com/pydio/services/idm/acl"
	"go.uber.org/zap"
)

var (
	apiACLFlags = []cli.Flag{}

	apiACLCmd = cli.Command{
		Name:   "acl",
		Usage:  "Starts gateway API for acl service.",
		Flags:  apiACLFlags,
		Action: mainAPIACL,
	}
)

func mainAPIACL(ctx *cli.Context) {

	serv, err := acl.NewAPIService(ctx)

	if err != nil {
		log.Logger(context.Background()).Fatal("Error creating ACL", zap.Error(err))
	}

	if err := serv.Run(); err != nil {
		log.Logger(context.Background()).Fatal("Error running ACL", zap.Error(err))
	}
}
