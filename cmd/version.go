package cmd

import (
	"fmt"

	cmdutil "github.com/leg100/stok/cmd/util"
	"github.com/leg100/stok/version"
	"github.com/spf13/cobra"
)

func versionCmd(opts *cmdutil.Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print client version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintf(opts.Out, "stok version %s\t%s\n", version.Version, version.Commit)
		},
	}
	return cmd
}
