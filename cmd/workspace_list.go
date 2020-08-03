package cmd

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/leg100/stok/api/v1alpha1"
	"github.com/leg100/stok/pkg/k8s"
	"github.com/leg100/stok/scheme"
	"github.com/spf13/cobra"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type listWorkspaceCmd struct {
	Path    string
	Context string

	out     io.Writer
	factory k8s.FactoryInterface
	cmd     *cobra.Command
}

func newListWorkspaceCmd(f k8s.FactoryInterface, out io.Writer) *cobra.Command {
	cc := &listWorkspaceCmd{}
	cc.cmd = &cobra.Command{
		Use:   "list",
		Short: "List all workspaces",
		RunE:  cc.doListWorkspace,
	}
	cc.cmd.Flags().StringVar(&cc.Path, "path", ".", "workspace config path")
	cc.cmd.Flags().StringVar(&cc.Context, "context", "", "Set kube context (defaults to kubeconfig current context)")

	// Add flags registered by imported packages (controller-runtime)
	cc.cmd.Flags().AddGoFlagSet(flag.CommandLine)

	cc.out = out
	cc.factory = f

	return cc.cmd
}

func (t *listWorkspaceCmd) doListWorkspace(cmd *cobra.Command, args []string) error {
	// Controller-runtime client for listing workspace resources
	rc, err := t.factory.NewClient(scheme.Scheme, t.Context)
	if err != nil {
		return err
	}

	currentNamespace, currentWorkspace, err := readEnvironmentFile(t.Path)
	// It's ok if there is no .terraform/environment file, so ignore not exist errors
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	err = t.listWorkspaces(rc, currentNamespace, currentWorkspace)
	if err != nil {
		return err
	}

	return nil
}

func (t *listWorkspaceCmd) listWorkspaces(rc client.Client, currentNamespace, currentWorkspace string) error {
	workspaces := v1alpha1.WorkspaceList{}
	// List across all namespaces
	if err := rc.List(context.TODO(), &workspaces); err != nil {
		return err
	}

	var prefix string
	for _, ws := range workspaces.Items {
		if ws.GetNamespace() == currentNamespace && ws.GetName() == currentWorkspace {
			prefix = "*"
		} else {
			prefix = ""
		}
		fmt.Fprintf(t.out, "%s\t%s/%s\n", prefix, ws.GetNamespace(), ws.GetName())
	}

	return nil
}
