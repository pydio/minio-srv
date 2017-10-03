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
	"github.com/pydio/services/data/source/objects"
)

var (
	dataSourceObjectsFlags = []cli.Flag{
		cli.StringFlag{
			Name:  "datasource",
			Usage: "Datasource name",
		},
		cli.StringFlag{
			Name:  "folder",
			Usage: "Start minio service on given folder",
		},
		cli.StringFlag{
			Name:  "gateway",
			Usage: "Start minio service as a gateway to a cloud storage (s3, azure, gcs). Use GATEWAY_ACCESS_KEY and GATEWAY_SECRET_KEY env for remote credentials.",
		},
		cli.StringFlag{
			Name:  "bucket",
			Usage: "When starting with a gateway, specify the name of the bucket that will be exposed",
		},
	}

	dataSourceObjectsCmd = cli.Command{
		Name:   "objects",
		Usage:  "Starts the objects service.",
		Flags:  dataSourceObjectsFlags,
		Action: mainDataSourceObjects,
	}
)

func mainDataSourceObjects(ctx *cli.Context) {

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

						// Retrieving configuration from the database
						config, err := utils.GetConfigsForService(common.SERVICE_OBJECTS_ + current)
						if err != nil {
							log.Logger(context.Background()).Error("Failed to get datasource config", zap.Error(err))
							break
						}

						cmdName := os.Args[0]
						cmdArgs := []string{"data", "source", "objects"}
						cmdArgs = append(cmdArgs, "--datasource", current)

						if service, ok := config.Get("service").(map[string]interface{}); ok {

							if folder, ok := service["folder"].(string); ok {
								cmdArgs = append(cmdArgs, "--folder", folder)
							}

							if gateway, ok := service["gateway"].(string); ok {
								cmdArgs = append(cmdArgs, "--gateway", gateway)
							}

							if bucket, ok := service["bucket"].(string); ok {
								cmdArgs = append(cmdArgs, "--bucket", bucket)
							}
						} else {
							log.Logger(context.Background()).Error("Failed to retrieve service config")
						}

						log.Logger(context.Background()).Info("Automatically starts server ", zap.String("datasource", common.SERVICE_OBJECTS_+current))

						startServiceAndWait(ctx, serviceForker(cmdName, cmdArgs), &wg)
					}
				}
			} else {
				log.Logger(context.Background()).Error("Failed to read datasource")
			}
		}
	} else {

		folder := ctx.String("folder")
		gateway := ctx.String("gateway")
		bucket := ctx.String("bucket")

		// We're in sinle datasource mode
		if folder == "" && gateway == "" && bucket == "" {
			log.Logger(context.Background()).Info("Please provide at least a path to the folder with --folder or a gateway with --gateway parameter and --bucket as bucket name")

			cli.ShowAppHelp(ctx)

			os.Exit(1)
		}

		if gateway != "" {
			folder = bucket
		}

		startServiceAndWait(ctx, objectsServiceStarter(datasource, folder, gateway), &wg)
	}

	wg.Wait()
}

func objectsServiceStarter(datasource string, folder string, gateway string) func(ctx *cli.Context) {
	return func(ctx *cli.Context) {
		srv, err := objects.NewObjectsService(context.Background(), datasource, folder, gateway)
		if err != nil {
			log.Logger(srv.Options().Context).Fatal("Error creating datasource objects", zap.Error(err))
		}

		if err := srv.Run(); err != nil {
			log.Logger(srv.Options().Context).Fatal("Error running datasource objects", zap.Error(err))
		}
	}
}
