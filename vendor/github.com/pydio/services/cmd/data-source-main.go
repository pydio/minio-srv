package cmd

import (
	"os"
	"sync"
	"time"

	"github.com/micro/cli"
)

// global flags for minio.
var (
	dataSourceFlags = mergeFlags(dataSourceIndexCmd.Flags, dataSourceObjectsCmd.Flags, dataSourceSyncCmd.Flags)

	dataSourceCmd = cli.Command{
		Name:   "source",
		Usage:  "Starts a datasource service.",
		Flags:  dataSourceFlags,
		Action: mainDataSource,

		Subcommands: []cli.Command{dataSourceIndexCmd, dataSourceObjectsCmd, dataSourceSyncCmd},
	}
)

func mainDataSource(ctx *cli.Context) {

	var wg sync.WaitGroup

	if ctx.Bool("help") {
		cli.ShowCommandHelp(ctx, "")
		os.Exit(1)
	}

	startServiceAndWait(ctx, mainDataSourceObjects, &wg)
	<-time.After(3 * time.Second)
	startServiceAndWait(ctx, mainDataSourceIndex, &wg)
	<-time.After(3 * time.Second)
	startServiceAndWait(ctx, mainDataSourceSync, &wg)

	wg.Wait()
}
