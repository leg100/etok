package generate

import (
	"github.com/leg100/stok/pkg/app"
	"github.com/spf13/cobra"
)

func GenerateCmd(opts *app.Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate deployment resources",
	}

	crdCmd, _ := GenerateCRDCmd(opts)
	cmd.AddCommand(crdCmd)

	operatorCmd, _ := GenerateOperatorCmd(opts)
	cmd.AddCommand(operatorCmd)

	return cmd
}
