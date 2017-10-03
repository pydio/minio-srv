package cmd

import (
	"os"
	"sync"

	"github.com/micro/cli"
	"time"
)

// API flags for Pydio.
var (
	schedulerFlags = []cli.Flag{}

	schedulerCmd = cli.Command{
		Name:  "scheduler",
		Usage: "Start scheduler-related services",
		Flags: schedulerFlags,

		Subcommands: []cli.Command{
			schedulerAllCmd,
			schedulerJobsCmd,
			schedulerTimerCmd,
			schedulerTasksCmd,
		},
	}

	schedulerAllFlags = mergeFlags(
		jobsFlags,
		timerFlags,
	)

	schedulerAllCmd = cli.Command{
		Name:   "all",
		Usage:  "Starts all scheduler services",
		Flags:  schedulerAllFlags,
		Action: mainSchedulerAll,
	}
)

func mainSchedulerAll(ctx *cli.Context) {

	var wg sync.WaitGroup

	if ctx.Bool("help") {
		cli.ShowCommandHelp(ctx, "")
		os.Exit(1)
	}

	startServiceAndWait(ctx, schedulerJobsCmd.Action, &wg)
	time.Sleep(3 * time.Second)
	startServiceAndWait(ctx, schedulerTimerCmd.Action, &wg)
	startServiceAndWait(ctx, schedulerTasksCmd.Action, &wg)

	wg.Wait()
}
