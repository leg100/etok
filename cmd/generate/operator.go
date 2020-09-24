package generate

import (
	"os"

	"github.com/leg100/stok/pkg/generate"
	"github.com/leg100/stok/version"
	"github.com/spf13/cobra"
)

func newOperatorCmd() *cobra.Command {
	operator := &generate.Operator{}

	cmd := &cobra.Command{
		Use:   "operator",
		Short: "Generate operator's kubernetes resources",
		RunE: func(cmd *cobra.Command, args []string) error {
			return operator.Generate(os.Stdout)
		},
	}

	cmd.Flags().StringVar(&operator.Name, "name", "stok-operator", "Name for kubernetes resources")
	cmd.Flags().StringVar(&operator.Namespace, "namespace", "default", "Kubernetes namespace for resources")
	cmd.Flags().StringVar(&operator.Image, "image", version.Image, "Docker image used for both the operator and the runner")

	return cmd
}
