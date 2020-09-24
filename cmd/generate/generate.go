package generate

import (
	"io"

	"github.com/spf13/cobra"
)

func GenerateCmd(out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate stok kubernetes resources",
	}
	cmd.AddCommand(newOperatorCmd(), newCrdsCmd(out))

	return cmd
}
