package cmd

import (
	"bufio"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"sync"

	"go.uber.org/zap"

	"github.com/micro/cli"
	"github.com/micro/go-micro/cmd"
	"github.com/minio/mc/pkg/console"
	"github.com/minio/minio/pkg/trie"
	"github.com/minio/minio/pkg/words"
	"github.com/pydio/services/common/log"
)

// global flags for minio.
var (
	runningDir  = ""
	globalFlags = []cli.Flag{
		cli.BoolFlag{
			Name:  "quiet",
			Usage: "Disable startup information.",
		},
	}
)

func init() {
	var err error

	runningDir, err = filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		log.Logger(context.Background()).Fatal("Error during initialisaiation", zap.Error(err))
	}

	defaultConfigFile = filepath.Join(runningDir, defaultConfigFile)
}

func newApp(name string) *cli.App {

	// Collection of minio commands currently supported are.
	commands := []cli.Command{}

	// Collection of minio commands currently supported in a trie tree.
	commandsTree := trie.NewTrie()

	// registerCommand registers a cli command.
	registerCommand := func(command cli.Command) {
		commands = append(commands, command)
		commandsTree.Insert(command.Name)
	}

	findClosestCommands := func(command string) []string {
		var closestCommands []string
		for _, value := range commandsTree.PrefixMatch(command) {
			closestCommands = append(closestCommands, value.(string))
		}

		sort.Strings(closestCommands)
		// Suggest other close commands - allow missed, wrongly added and
		// even transposed characters
		for _, value := range commandsTree.Walk(commandsTree.Root()) {
			if sort.SearchStrings(closestCommands, value.(string)) < len(closestCommands) {
				continue
			}
			// 2 is arbitrary and represents the max
			// allowed number of typed errors
			if words.DamerauLevenshteinDistance(command, value.(string)) < 2 {
				closestCommands = append(closestCommands, value.(string))
			}
		}

		return closestCommands
	}

	// Register all commands.
	registerCommand(allCmd)
	registerCommand(configCmd)
	registerCommand(idmCmd)
	registerCommand(dataCmd)
	registerCommand(searchCmd)
	registerCommand(apiCmd)
	registerCommand(activityCmd)
	registerCommand(versionCmd)
	registerCommand(schedulerCmd)

	// Set up app.
	cli.HelpFlag = cli.BoolFlag{
		Name:  "help, h",
		Usage: "Show help.",
	}

	app := cli.NewApp()

	app.Name = name
	app.Author = "Pydio"
	app.Version = Version
	app.Usage = "Cloud Storage Server."
	// app.Description = `Pydio is a comprehensive sync & share solution for your collaborators. Open-source software deployed on-premise or in a private cloud.`
	app.Flags = append(cmd.DefaultFlags, globalFlags...)
	app.HideVersion = true // Hide `--version` flag, we already have `minio version`.
	// app.HideHelpCommand = true // Hide `help, h` command, we already have `minio --help`.

	app.Commands = commands
	// app.CustomAppHelpTemplate = pydioHelpTemplate
	app.CommandNotFound = func(ctx *cli.Context, command string) {
		console.Printf("‘%s’ is not a pydio sub-command. See ‘pydio --help’.\n", command)
		closestCommands := findClosestCommands(command)
		if len(closestCommands) > 0 {
			console.Println()
			console.Println("Did you mean one of these?")
			for _, cmd := range closestCommands {
				console.Printf("\t‘%s’\n", cmd)
			}
		}

		os.Exit(1)
	}

	return app
}

func startServiceAndWait(ctx *cli.Context, fn func(c *cli.Context), wg *sync.WaitGroup) {
	wg.Add(1)

	go func() {
		defer wg.Done()

		fn(ctx)
	}()
}

func serviceForker(name string, args []string) func(ctx *cli.Context) {
	return func(ctx *cli.Context) {

		cmd := exec.Command(name, args...)
		out, _ := cmd.StdoutPipe()
		err, _ := cmd.StderrPipe()

		outScanner := bufio.NewScanner(out)
		errScanner := bufio.NewScanner(err)

		go func() {
			for outScanner.Scan() {
				log.Logger(nil).Info(outScanner.Text())
			}
		}()

		go func() {
			for errScanner.Scan() {
				log.Logger(nil).Info(errScanner.Text())
			}
		}()

		cmd.Start()

		cmd.Wait()
	}
}

func mergeFlags(flagsArr ...[]cli.Flag) []cli.Flag {

	len := 0
	flagMap := make(map[string]cli.Flag)
	for _, flags := range flagsArr {
		for _, flag := range flags {
			if _, ok := flagMap[flag.GetName()]; !ok {
				flagMap[flag.GetName()] = flag
				len++
			}
		}
	}

	var ret []cli.Flag
	for _, v := range flagMap {
		ret = append(ret, v)
	}

	return ret
}

// Main main for minio server.
func Main(args []string) {
	// Set the minio app name.
	appName := filepath.Base(args[0])

	// Run the app - exit on error.
	if err := newApp(appName).Run(args); err != nil {
		os.Exit(1)
	}
}
