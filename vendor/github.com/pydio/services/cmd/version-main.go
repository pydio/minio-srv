package cmd

import (
	"os"

	"github.com/micro/cli"
	"github.com/minio/mc/pkg/console"
)

var versionCmd = cli.Command{
	Name:   "version",
	Usage:  "Print version.",
	Action: mainVersion,
}

func mainVersion(ctx *cli.Context) {
	if len(ctx.Args()) != 0 {
		cli.ShowCommandHelp(ctx, "version")
		os.Exit(1)
	}

	console.Println("Version: " + Version)
	console.Println("Release-Tag: " + ReleaseTag)
	console.Println("Commit-ID: " + CommitID)
}
