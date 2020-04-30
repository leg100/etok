package cmd

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// debugCmd represents the debug command
var debugCmd = &cobra.Command{
	Use:   "debug",
	Short: "Show configuration options",
	Run: func(cmd *cobra.Command, args []string) {
		// TODO: Consider using viper.AllSettings()
		logger.Infow("Dump of configuration values",
			"workspace", viper.GetString("workspace"),
			"namespace", viper.GetString("namespace"),
			"configFile", viper.ConfigFileUsed(),
		)
	},
}

func init() {
	rootCmd.AddCommand(debugCmd)
}
