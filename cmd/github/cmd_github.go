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

	deploy, _ := deployCmd(f)
	cmd.AddCommand(deploy)

	run, _ := runCmd(f)
	cmd.AddCommand(run)

	return cmd
}
