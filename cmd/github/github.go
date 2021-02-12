package github

import (
	cmdutil "github.com/leg100/etok/cmd/util"
	"github.com/spf13/cobra"
)

const (
	// default namespace github app is deployed to
	defaultNamespace = "github"
)

func GithubCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "github",
		Short: "Manage github apps",
	}

	cc, _ := createCmd(f)
	cmd.AddCommand(cc)

	return cmd
}
