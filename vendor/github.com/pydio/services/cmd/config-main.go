package cmd

import (
	"context"

	"go.uber.org/zap"

	"github.com/micro/cli"
	"github.com/pydio/services/common/log"
	"github.com/pydio/services/config"
)

var (
	defaultConfigFile = "config/file/sample.json"

	configFlags = []cli.Flag{
		cli.StringFlag{
			Name:   "file",
			EnvVar: "PYDIO_CONFIG_FILE",
			Usage:  "Path to a json or yaml file containing the basic configuration for services",
		},
	}

	configCmd = cli.Command{
		Name:   "config",
		Usage:  "Starts the configs service.",
		Flags:  configFlags,
		Action: mainConfig,
	}
)

func mainConfig(ctx *cli.Context) {

	filename := ctx.String("file")
	if len(filename) == 0 {
		filename = defaultConfigFile
	}

	configService := config.NewConfigMicroService(filename)
	if err := configService.Run(); err != nil {
		log.Logger(context.Background()).Fatal("Error running config", zap.Error(err))
	}
}
