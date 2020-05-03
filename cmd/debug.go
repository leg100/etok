package cmd

import (
	"github.com/apex/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// debugCmd represents the debug command
var debugCmd = &cobra.Command{
	Use:   "debug",
	Short: "Show configuration options",
	Run: func(cmd *cobra.Command, args []string) {
		// TODO: Consider using viper.AllSettings()
		log.WithFields(log.Fields{
			"workspace":  viper.GetString("workspace"),
			"namespace":  viper.GetString("namespace"),
			"configFile": viper.ConfigFileUsed(),
		}).Info("Dump of configuration values")
	},
}

func init() {
	rootCmd.AddCommand(debugCmd)
}
