package cmd

import (
	"context"

	"github.com/micro/cli"
	"github.com/pydio/services/common/log"
	"github.com/pydio/services/idm/acl"
	"go.uber.org/zap"
)

var aclCmd = cli.Command{
	Name:   "acl",
	Usage:  "Starts the acl service.",
	Action: mainACL,
}

func mainACL(ctx *cli.Context) {
	serv := acl.NewMicroService(context.Background())

	if err := serv.Run(); err != nil {
		log.Logger(context.Background()).Fatal("Error running ACL", zap.Error(err))
	}
}
