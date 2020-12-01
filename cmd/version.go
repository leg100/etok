package cmd

import (
	"fmt"

	cmdutil "github.com/leg100/etok/cmd/util"
	"github.com/leg100/etok/pkg/version"
	"github.com/spf13/cobra"
)

func versionCmd(opts *cmdutil.Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print client version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintf(opts.Out, "etok version %s\t%s\n", version.Version, version.Commit)
		},
	}
	return cmd
}
