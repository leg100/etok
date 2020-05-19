package cmd

import (
	"os"
	"time"

	"github.com/apex/log"
	"github.com/leg100/stok/logging/handlers/cli"
	"github.com/leg100/stok/logging/handlers/prefix"
	"github.com/spf13/cobra"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/viper"
)

//go:generate go run generate.go

var (
	cfgFile string
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:              "stok [command] -- [terraform args]",
	PersistentPreRun: validate,
	Short:            "Supercharge terraform on kubernetes",
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig, initLogging)

	rootCmd.DisableFlagsInUseLine = true

	rootCmd.PersistentFlags().String("config", "", "config file (default is $HOME/.stok.yaml)")
	rootCmd.PersistentFlags().String("loglevel", "info", "logging verbosity level")
	rootCmd.PersistentFlags().String("path", ".", "path containing terraform config files")
	rootCmd.PersistentFlags().String("namespace", "default", "kubernetes namespace")
	rootCmd.PersistentFlags().String("workspace", "default", "terraform workspace")
	rootCmd.PersistentFlags().Duration("timeout-pod", time.Minute, "timeout for pod to be ready and running")
	rootCmd.PersistentFlags().Duration("timeout-client", 10*time.Second, "timeout for client to signal readiness")
	rootCmd.PersistentFlags().Duration("timeout-queue", time.Hour, "timeout waiting in workspace queue")

	viper.BindPFlags(rootCmd.PersistentFlags())
}

func initLogging() {
	log.SetHandler(prefix.New(cli.New(os.Stdout, os.Stderr), "[stok] "))
	log.SetLevelFromString(viper.GetString("loglevel"))
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := homedir.Dir()
		if err != nil {
			log.WithError(err).Error("")
			os.Exit(1)
		}

		// Search config in home directory with name ".stok" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigName(".stok")
	}

	viper.SetEnvPrefix("stok")
	viper.AutomaticEnv() // read in environment variables that match

	viper.ReadInConfig()
}

func validate(cmd *cobra.Command, args []string) {
	if _, err := os.Stat(viper.GetString("path")); err != nil {
		log.Errorf("error reading path: %v\n", err)
		os.Exit(10)
	}
}
