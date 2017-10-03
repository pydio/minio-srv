package cmd

import (
	"os"
	"sync"

	"github.com/micro/cli"
)

// global flags for minio.
var (
	dataFlags = mergeFlags(dataSourceCmd.Flags, dataTreeCmd.Flags, docStoreFlags)

	dataCmd = cli.Command{
		Name:   "data",
		Usage:  "Starts all data-related services.",
		Flags:  dataFlags,
		Action: mainData,

		Subcommands: []cli.Command{dataSourceCmd, dataTreeCmd, dataEncryptionCmd, dataMetaCmd, dataDocStoreCmd, dataVersionCmd},
	}
)

func mainData(ctx *cli.Context) {

	var wg sync.WaitGroup

	if ctx.Bool("help") {
		cli.ShowCommandHelp(ctx, "")
		os.Exit(1)
	}

	startServiceAndWait(ctx, dataSourceCmd.Action, &wg)
	startServiceAndWait(ctx, dataTreeCmd.Action, &wg)
	startServiceAndWait(ctx, dataMetaCmd.Action, &wg)
	startServiceAndWait(ctx, dataEncryptionCmd.Action, &wg)
	//startServiceAndWait(ctx, dataVersionCmd.Action, &wg)
	startServiceAndWait(ctx, dataDocStoreCmd.Action, &wg)

	wg.Wait()
}
