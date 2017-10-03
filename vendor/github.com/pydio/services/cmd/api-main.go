package cmd

import (
	"context"
	"os"
	"sync"

	"github.com/micro/cli"
	"github.com/pydio/services/common"
	"github.com/pydio/services/common/log"
)

// API flags for Pydio.
var (
	apiFlags = []cli.Flag{}

	apiCmd = cli.Command{
		Name:  "api",
		Usage: "Start one or all gateway API's.",
		Flags: apiFlags,

		Subcommands: []cli.Command{
			apiAllCmd,
			apiSearchCmd,
			apiMetaCmd,
			apiTreeCmd,
			apiS3Cmd,
			apiDavCmd,
			apiWopiCmd,
			apiWebSocketCmd,
			apiActivityCmd,
			apiJobsCmd,
			apiACLCmd,
			apiUserCmd,
			apiRoleCmd,
			apiWorkspaceCmd,
			apiConfigCmd,
			apiDocStoreCmd,
		},
	}

	apiAllFlags = mergeFlags(
		apiSearchFlags,
		apiMetaFlags,
		apiTreeFlags,
		apiS3Flags,
		apiWebSocketFlags,
		apiActivityFlags,
		apiJobsFlags,
		apiACLFlags,
		apiUserFlags,
		apiRoleFlags,
		apiWorkspaceFlags,
		apiConfigFlags,
		apiDocStoreFlags,
	)

	apiAllCmd = cli.Command{
		Name:   "all",
		Usage:  "Starts all gateway API's.",
		Flags:  apiAllFlags,
		Action: mainApiAll,
	}
)

func mainApiAll(ctx *cli.Context) {

	var wg sync.WaitGroup

	if ctx.Bool("help") {
		cli.ShowCommandHelp(ctx, "")
		os.Exit(1)
	}

	log.Logger(context.Background()).Info(
		"\n***********************************" +
			"\nStarting API Services: make sure to run " +
			"\n the following command to complete gateway:" +
			"\nmicro --client=grpc api --namespace=" + common.SERVICE_API_NAMESPACE_ +
			"\n***********************************",
	)

	startServiceAndWait(ctx, apiSearchCmd.Action, &wg)
	startServiceAndWait(ctx, apiMetaCmd.Action, &wg)
	startServiceAndWait(ctx, apiTreeCmd.Action, &wg)
	startServiceAndWait(ctx, apiS3Cmd.Action, &wg)
	startServiceAndWait(ctx, apiWebSocketCmd.Action, &wg)
	startServiceAndWait(ctx, apiACLCmd.Action, &wg)
	startServiceAndWait(ctx, apiActivityCmd.Action, &wg)
	startServiceAndWait(ctx, apiJobsCmd.Action, &wg)
	startServiceAndWait(ctx, apiUserCmd.Action, &wg)
	startServiceAndWait(ctx, apiRoleCmd.Action, &wg)
	startServiceAndWait(ctx, apiWorkspaceCmd.Action, &wg)
	startServiceAndWait(ctx, apiConfigCmd.Action, &wg)
	startServiceAndWait(ctx, apiDocStoreCmd.Action, &wg)

	wg.Wait()
}
