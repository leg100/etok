package cmd

import (
	"context"
	"fmt"
	"io"

	"github.com/leg100/stok/pkg/apis"
	"github.com/leg100/stok/pkg/apis/stok/v1alpha1"
	"github.com/leg100/stok/pkg/k8s"
	"github.com/spf13/cobra"
	"k8s.io/kubectl/pkg/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type listWorkspaceCmd struct {
	Path           string
	KubeConfigPath string

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
	cc.cmd.Flags().StringVar(&cc.KubeConfigPath, "kubeconfig", "", "absolute path to kubeconfig file (default is $HOME/.kube/config)")

	cc.out = out
	cc.factory = f

	return cc.cmd
}

func (t *listWorkspaceCmd) doListWorkspace(cmd *cobra.Command, args []string) error {
	config, err := k8s.ConfigFromPath(t.KubeConfigPath)
	if err != nil {
		return err
	}

	// Get built-in scheme
	s := scheme.Scheme
	// And add our CRDs
	apis.AddToScheme(s)

	// Controller-runtime client for constructing workspace resource
	rc, err := t.factory.NewClient(config, s)
	if err != nil {
		return err
	}

	currentNamespace, currentWorkspace, err := readEnvironmentFile(t.Path)
	if err != nil {
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
