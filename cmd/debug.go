package cmd

import (
	log "github.com/sirupsen/logrus"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// debugCmd represents the debug command
var debugCmd = &cobra.Command{
	Use:   "debug",
	Short: "Show configuration options",
	Run: func(cmd *cobra.Command, args []string) {
		// Only log the debug severity or above.
		log.SetLevel(log.DebugLevel)

		// TODO: Consider using viper.AllSettings()

		log.WithFields(log.Fields{
			"workspace":  viper.GetString("workspace"),
			"namespace":  viper.GetString("namespace"),
			"configFile": viper.ConfigFileUsed(),
		}).Debug("Dump of configuration values")
	},
}

func init() {
	rootCmd.AddCommand(debugCmd)
}
