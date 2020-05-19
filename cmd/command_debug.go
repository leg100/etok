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
		log.WithFields(log.Fields(viper.AllSettings())).Info("Config dump")
	},
}

func init() {
	rootCmd.AddCommand(debugCmd)
}
