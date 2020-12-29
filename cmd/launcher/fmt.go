package launcher

import (
	"github.com/leg100/etok/cmd/runner"
	"github.com/spf13/cobra"
)

func FmtCmd(exec runner.Executor) *cobra.Command {
	return &cobra.Command{
		Use:   "fmt",
		Short: "Run terraform fmt",
		RunE: func(cmd *cobra.Command, args []string) error {
			return exec.Execute(cmd.Context(), append([]string{"terraform", "fmt"}, args...))
		},
	}
}
