package cmd

import (
	"os"
	"sync"

	"github.com/micro/cli"
)

// global flags for minio.
var (
	idmFlags = mergeFlags(aclCmd.Flags, roleCmd.Flags, userCmd.Flags, authCmd.Flags, workspaceCmd.Flags)

	idmCmd = cli.Command{
		Name:        "idm",
		Usage:       "Starts an Identity Management service.",
		Flags:       idmFlags,
		Action:      mainIDM,
		Subcommands: []cli.Command{aclCmd, roleCmd, userCmd, authCmd, workspaceCmd},
	}
)

func mainIDM(ctx *cli.Context) {

	var wg sync.WaitGroup

	if ctx.Bool("help") {
		cli.ShowCommandHelp(ctx, "")
		os.Exit(1)
	}

	startServiceAndWait(ctx, mainACL, &wg)
	startServiceAndWait(ctx, mainRole, &wg)
	startServiceAndWait(ctx, mainUser, &wg)
	startServiceAndWait(ctx, authCmd.Action, &wg)
	startServiceAndWait(ctx, workspaceCmd.Action, &wg)

	wg.Wait()
}
