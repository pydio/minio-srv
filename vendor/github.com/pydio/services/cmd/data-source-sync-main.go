package cmd

import (
	"context"
	"os"
	"sync"

	"github.com/micro/cli"
	"github.com/pydio/services/common"
	"github.com/pydio/services/common/log"
	"github.com/pydio/services/common/utils"
	datasourcesync "github.com/pydio/services/data/source/sync"
	"go.uber.org/zap"
)

var (
	dataSourceSyncFlags = []cli.Flag{
		cli.StringFlag{
			Name:  "datasource",
			Usage: "Datasource name",
		},
		cli.StringFlag{
			Name:  "watch",
			Usage: "Directly watch the underlying FS instead of listening to s3 event. Provide a path to the folder.",
		},
		cli.BoolFlag{
			Name:  "normalize",
			Usage: "If S3 is leaving on an NFD file system, normalize inputs and outputs to NFC",
		},
	}

	dataSourceSyncCmd = cli.Command{
		Name:   "sync",
		Usage:  "Starts the sync service.",
		Flags:  dataSourceSyncFlags,
		Action: mainDataSourceSync,
	}
)

func mainDataSourceSync(ctx *cli.Context) {

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
						log.Logger(context.Background()).Info("Automatically starts server ", zap.String("datasource", common.SERVICE_SYNC_+current))

						cmdName := os.Args[0]
						cmdArgs := []string{"data", "source", "sync"}
						cmdArgs = append(cmdArgs, "--datasource", current)

						if service, ok := config.Get("service").(map[string]interface{}); ok {
							if normalize, ok := service["normalize"].(bool); ok && normalize {
								cmdArgs = append(cmdArgs, "--normalize")
							}

							if watch, ok := service["watch"].(string); ok {
								cmdArgs = append(cmdArgs, "--watch", watch)
							}
						}

						startServiceAndWait(ctx, serviceForker(cmdName, cmdArgs), &wg)
					}
				}
			} else {
				log.Logger(context.Background()).Error("Failed to read datasource")
			}
		}
	} else {
		watch := ctx.String("watch")
		normalize := ctx.Bool("normalize")

		startServiceAndWait(ctx, syncServiceStarter(datasource, normalize, watch), &wg)
	}

	wg.Wait()
}

func syncServiceStarter(datasource string, normalize bool, watch string) func(ctx *cli.Context) {
	return func(ctx *cli.Context) {
		serv, err := datasourcesync.NewSyncService(context.Background(), datasource, normalize, watch)

		if err != nil {
			log.Logger(context.Background()).Fatal("Error creating datasource index", zap.Error(err))
		}

		if err := serv.Run(); err != nil {
			log.Logger(context.Background()).Fatal("Error running datasource index", zap.Error(err))
		}
	}
}
