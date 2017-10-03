package cmd

import (
	"context"

	"github.com/micro/cli"
	"github.com/pydio/services/common/log"
	"github.com/pydio/services/data/encryption"
	"go.uber.org/zap"
)

var (
	encFlags = []cli.Flag{}

	dataEncryptionCmd = cli.Command{
		Name:   "encryption",
		Usage:  "Starts the encryption key server.",
		Flags:  encFlags,
		Action: mainEncryption,
	}
)

func mainEncryption(ctx *cli.Context) {

	serv, err := encryption.StartEncryptionKeyService(context.Background())

	if err != nil {
		log.Logger(context.Background()).Fatal("Error creating encryption", zap.Error(err))
	}

	if err := serv.Run(); err != nil {
		log.Logger(context.Background()).Fatal("Error running encryption", zap.Error(err))
	}
}
