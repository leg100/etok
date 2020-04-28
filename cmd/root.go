package cmd

import (
	"flag"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/viper"
)

var (
	cfgFile     string
	workspace   string
	namespace   string
	loglevel    *zapcore.Level
	logger      *zap.SugaredLogger
	podWaitTime string
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

	loglevel = zap.LevelFlag("loglevel", zapcore.InfoLevel, "logging verbosity level")
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	viper.BindPFlags(pflag.CommandLine)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.stok.yaml)")
	rootCmd.PersistentFlags().StringVar(&namespace, "namespace", "default", "kubernetes namespace")
	rootCmd.PersistentFlags().StringVar(&workspace, "workspace", "default", "terraform workspace")
	rootCmd.PersistentFlags().StringVar(&podWaitTime, "pod-timeout", "10s", "pod wait timeout")

	viper.BindPFlag("namespace", rootCmd.PersistentFlags().Lookup("namespace"))
	viper.BindPFlag("workspace", rootCmd.PersistentFlags().Lookup("workspace"))
}

func initLogging() {
	config := zap.NewProductionConfig()
	config.Level.SetLevel(*loglevel)
	config.DisableStacktrace = true
	config.DisableCaller = true

	l, err := config.Build()
	if err != nil {
		panic(err.Error())
	}
	logger = l.Sugar()
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
			logger.Error(err)
			logger.Sync()
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
