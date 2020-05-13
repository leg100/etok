package cmd

import (
	"os"

	"github.com/apex/log"
	"github.com/leg100/stok/logging/handlers/cli"
	"github.com/leg100/stok/logging/handlers/prefix"
	"github.com/spf13/cobra"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/viper"
)

//go:generate go run generate.go

var (
	cfgFile      string
	workspace    string
	namespace    string
	loglevel     string
	podWaitTime  string
	path         string
	queueTimeout int
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "stok [command] -- [terraform args]",
	Short: "Supercharge terraform on kubernetes",
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

	rootCmd.PersistentFlags().StringVar(&loglevel, "loglevel", "info", "logging verbosity level")
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.stok.yaml)")
	rootCmd.PersistentFlags().StringVar(&path, "path", ".", "path containing terraform config files")
	rootCmd.PersistentFlags().StringVar(&namespace, "namespace", "default", "kubernetes namespace")
	rootCmd.PersistentFlags().StringVar(&workspace, "workspace", "default", "terraform workspace")
	rootCmd.PersistentFlags().StringVar(&podWaitTime, "pod-timeout", "10s", "pod wait timeout")
	rootCmd.PersistentFlags().IntVar(&queueTimeout, "queue-timeout", 60, "queue timeout in seconds")

	viper.BindPFlag("namespace", rootCmd.PersistentFlags().Lookup("namespace"))
	viper.BindPFlag("workspace", rootCmd.PersistentFlags().Lookup("workspace"))
	viper.BindPFlag("path", rootCmd.PersistentFlags().Lookup("path"))
}

func initLogging() {
	log.SetHandler(prefix.New(cli.New(os.Stdout, os.Stderr), "[stok] "))
	log.SetLevelFromString(loglevel)
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

func validatePath(cmd *cobra.Command, args []string) {
	if _, err := os.Stat(path); err != nil {
		log.Errorf("error reading path: %v\n", err)
		os.Exit(10)
	}
}
