package cmd

import (
	"context"
	"os"
	"sync"

	"go.uber.org/zap"

	"github.com/micro/cli"
	"github.com/pydio/services/common"
	"github.com/pydio/services/common/log"
	"github.com/pydio/services/common/utils"
	"github.com/pydio/services/data/source/index"
)

var (
	dataSourceIndexFlags = []cli.Flag{
		cli.StringFlag{
			Name:  "datasource",
			Usage: "Datasource name",
		},
	}

	dataSourceIndexCmd = cli.Command{
		Name:   "index",
		Usage:  "Starts the role service.",
		Flags:  dataSourceIndexFlags,
		Action: mainDataSourceIndex,
	}
)

func mainDataSourceIndex(ctx *cli.Context) {

	var wg sync.WaitGroup

	datasource := ctx.String("datasource")

	if len(datasource) == 0 {
		// This means we must start it all
		config, err := utils.GetConfigsForService(common.SERVICE_TREE)
		if err != nil {
			return
		}

		v := config.Get("datasources")

		if v != nil {
			if datasources, ok := v.([]interface{}); ok {
				for _, vv := range datasources {
					if current, ok := vv.(string); ok {
						log.Logger(context.Background()).Info("Automatically starts server ", zap.String("datasource", common.SERVICE_INDEX_+current))

						cmdName := os.Args[0]
						cmdArgs := []string{"data", "source", "index"}
						cmdArgs = append(cmdArgs, "--datasource", current)

						startServiceAndWait(ctx, serviceForker(cmdName, cmdArgs), &wg)
					}
				}
			} else {
				log.Logger(context.Background()).Error("Failed to read datasource")
			}
		}
	} else {
		startServiceAndWait(ctx, indexServiceStarter(datasource), &wg)
	}

	wg.Wait()

}

func indexServiceStarter(datasource string) func(ctx *cli.Context) {
	return func(ctx *cli.Context) {
		serv, err := index.NewIndexationService(context.Background(), datasource)

		if err != nil {
			log.Logger(context.Background()).Fatal("Error creating datasource index", zap.Error(err))
		}

		if err := serv.Run(); err != nil {
			log.Logger(context.Background()).Fatal("Error running datasource index", zap.Error(err))
		}
	}
}
