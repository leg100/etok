package cmd

import (
	"github.com/spf13/cobra"
)

// shellCmd represents the shell command
var shellCmd = &cobra.Command{
	Use:   "shell -- [args]",
	Short: "Run interactive shell on workspace pod",
	Run: func(cmd *cobra.Command, args []string) {
		dashArgs := getArgsAfterDash(cmd, args)
		runApp("sh", dashArgs)
	},
}

func init() {
	rootCmd.AddCommand(shellCmd)
}
