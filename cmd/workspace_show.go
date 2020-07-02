package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

type showWorkspaceCmd struct {
	Path string

	cmd *cobra.Command
}

func newShowWorkspaceCmd() *cobra.Command {
	cc := &showWorkspaceCmd{}
	cc.cmd = &cobra.Command{
		Use:   "show",
		Short: "Show current stok workspace",
		Long:  "Show the current stok workspace for this module",
		RunE:  cc.doShowWorkspace,
	}
	cc.cmd.Flags().StringVar(&cc.Path, "path", ".", "workspace config path")

	return cc.cmd
}

func (t *showWorkspaceCmd) doShowWorkspace(cmd *cobra.Command, args []string) error {
	namespace, workspace, err := readEnvironmentFile(t.Path)
	if err != nil {
		return err
	}

	fmt.Printf("%s/%s\n", namespace, workspace)

	return nil
}
