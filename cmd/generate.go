package cmd

import (
	"github.com/spf13/cobra"
)

func generateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate stok kubernetes resources",
	}
	cmd.AddCommand(newOperatorCmd(), newCrdsCmd())

	return cmd
}
