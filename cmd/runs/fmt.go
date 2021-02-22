package runs

import (
	"github.com/leg100/etok/pkg/executor"
	"github.com/spf13/cobra"
)

func FmtCmd(exec executor.Executor) *cobra.Command {
	return &cobra.Command{
		Use:   "fmt",
		Short: "Run terraform fmt",
		RunE: func(cmd *cobra.Command, args []string) error {
			return exec.Execute(cmd.Context(), append([]string{"terraform", "fmt"}, args...))
		},
	}
}
