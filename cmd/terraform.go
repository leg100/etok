package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/leg100/stok/app"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var terraformCommands = []string{
	"apply",
	//"console",
	"destroy",
	//"env",
	//"fmt",
	"get",
	//"graph",
	"import",
	"init",
	"output",
	"plan",
	//"providers",
	"refresh",
	"show",
	"taint",
	"untaint",
	"validate",
	"version",
	//"workspace",
	//"0.12upgrade",
	//"debug",
	"force-unlock",
	//"push",
	"state",
}

func init() {
	for _, c := range terraformCommands {
		var cc = &cobra.Command{
			Use:   fmt.Sprintf("%s [flags] -- [%s args]", c, c),
			Short: fmt.Sprintf("Run terraform %s", c),
			Run: func(cmd *cobra.Command, args []string) {
				dashArgs := append([]string{cmd.Name()}, getArgsAfterDash(cmd, args)...)

				// TODO: handle [DIR] positional argument
				runApp("terraform", dashArgs)
			},
		}

		rootCmd.AddCommand(cc)
	}
}

func runApp(cmd string, args []string) {
	// initialise both controller-runtime client and client-go client
	client, kubeClient, err := app.InitClient()
	if err != nil {
		logger.Error(err)
		logger.Sync()
		os.Exit(1)
	}

	podWaitDuration, err := time.ParseDuration(podWaitTime)
	if err != nil {
		logger.Error(err)
		logger.Sync()
		os.Exit(1)
	}

	app := &app.App{
		Namespace:      viper.GetString("namespace"),
		Workspace:      viper.GetString("workspace"),
		Command:        []string{cmd},
		Logger:         logger,
		Args:           args,
		Client:         *client,
		KubeClient:     kubeClient,
		PodWaitTimeout: podWaitDuration,
	}
	err = app.Run()
	if err != nil {
		logger.Error(err)
		logger.Sync()
		os.Exit(1)
	}
}

// extract args after '--' (if provided)
func getArgsAfterDash(cmd *cobra.Command, args []string) []string {
	if cmd.ArgsLenAtDash() > -1 {
		return args[cmd.ArgsLenAtDash():]
	} else {
		return []string{}
	}
}
