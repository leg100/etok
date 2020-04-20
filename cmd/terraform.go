package cmd

import (
	"fmt"
	"os"

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
				// extract terraform arguments after '--' (if provided)
				// TODO: handle [DIR] positional argument
				tfArgs := []string{cmd.Name()}
				if cmd.ArgsLenAtDash() > -1 {
					tfArgs = append(tfArgs, args[cmd.ArgsLenAtDash():]...)
				}

				// initialise both controller-runtime client and client-go client
				client, kubeClient, err := app.InitClient()
				if err != nil {
					fmt.Fprint(os.Stderr, err)
					os.Exit(1)
				}

				app := &app.App{
					Namespace:  viper.GetString("namespace"),
					Workspace:  viper.GetString("workspace"),
					Args:       tfArgs,
					Client:     *client,
					KubeClient: kubeClient,
				}
				err = app.Run()
				if err != nil {
					fmt.Fprint(os.Stderr, err)
					os.Exit(1)
				}
			},
		}

		rootCmd.AddCommand(cc)
	}
}
