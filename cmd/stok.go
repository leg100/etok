package cmd

import (
	"os"

	"github.com/apex/log"
	"github.com/leg100/stok/logging/handlers/cli"
	"github.com/leg100/stok/logging/handlers/prefix"
	"github.com/leg100/stok/version"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type stokCmd struct {
	Config     string
	Loglevel   string
	KubeConfig string

	cmd *cobra.Command
}

func allCommands() []*cobra.Command {
	return append(newTerraformCmds().getCommands(), workspaceCmd(), generateCmd())
}

func Execute(args []string) error {
	cc := newStokCmd()
	cc.cmd.AddCommand(allCommands()...)
	cc.cmd.SetArgs(args)

	return cc.cmd.Execute()
}

func newStokCmd() *stokCmd {
	cc := &stokCmd{}

	cc.cmd = &cobra.Command{
		Use:               "stok",
		Short:             "Supercharge terraform on kubernetes",
		PersistentPreRunE: cc.preRun,
		SilenceUsage:      true,
		Version:           version.Version,
	}

	cc.cmd.PersistentFlags().StringVar(&cc.Config, "config", "", "config file (default is $HOME/.stok.yaml)")
	cc.cmd.PersistentFlags().StringVar(&cc.Loglevel, "loglevel", "info", "logging verbosity level")

	return cc
}

func (cc *stokCmd) preRun(cmd *cobra.Command, args []string) error {
	if err := cc.initConfig(); err != nil {
		return err
	}

	if err := unmarshalV(cc); err != nil {
		return err
	}

	initLogging(cc)
	return nil
}

// initConfig reads in config file and ENV variables if set.
func (cc *stokCmd) initConfig() error {
	if cc.Config != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cc.Config)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}

		// Search config in home directory with name ".stok" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigName(".stok")
	}

	viper.SetEnvPrefix("stok")
	viper.AutomaticEnv() // read in environment variables that match

	viper.ReadInConfig()

	return nil
}

func initLogging(cmd *stokCmd) {
	log.SetHandler(prefix.New(cli.New(os.Stdout, os.Stderr), "[stok] "))
	log.SetLevelFromString(cmd.Loglevel)
}
