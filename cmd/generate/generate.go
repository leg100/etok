package generate

import (
	cmdutil "github.com/leg100/etok/cmd/util"
	"github.com/spf13/cobra"
)

func GenerateCmd(opts *cmdutil.Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate deployment resources",
	}

	crdCmd, _ := generateCRDCmd(opts)
	cmd.AddCommand(crdCmd)

	operatorCmd, _ := generateOperatorCmd(opts)
	cmd.AddCommand(operatorCmd)

	return cmd
}
