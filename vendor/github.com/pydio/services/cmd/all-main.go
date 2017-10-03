package cmd

import (
	"os"
	"sync"

	"github.com/micro/cli"
	"time"
)

// global flags for Pydio.
var (
	allFlags = mergeFlags(configCmd.Flags, dataCmd.Flags, idmCmd.Flags)

	allCmd = cli.Command{
		Name:   "all",
		Usage:  "Starts all services.",
		Flags:  allFlags,
		Action: mainAll,
	}
)

func mainAll(ctx *cli.Context) {

	var wg sync.WaitGroup

	if ctx.Bool("help") {
		cli.ShowCommandHelp(ctx, "")
		os.Exit(1)
	}

	startServiceAndWait(ctx, configCmd.Action, &wg)
	time.Sleep(3 * time.Second)
	startServiceAndWait(ctx, dataCmd.Action, &wg)
	startServiceAndWait(ctx, idmCmd.Action, &wg)
	startServiceAndWait(ctx, searchCmd.Action, &wg)

	wg.Wait()
}
