package workspace

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/leg100/stok/api/stok.goalspike.com/v1alpha1"
	"github.com/leg100/stok/pkg/k8s"
	"github.com/leg100/stok/util"
	"github.com/spf13/cobra"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ListWorkspace struct {
	Path    string
	Context string

	Out     io.Writer
	Factory k8s.FactoryInterface
	Cmd     *cobra.Command
}

func (t *ListWorkspace) Run(ctx context.Context) error {
	config, err := t.Factory.NewConfig(t.Context)
	if err != nil {
		return fmt.Errorf("failed to obtain kubernetes client config: %w", err)
	}

	// Controller-runtime client for listing workspace resources
	rc, err := t.Factory.NewClient(config)
	if err != nil {
		return fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	currentNamespace, currentWorkspace, err := util.ReadEnvironmentFile(t.Path)
	// It's ok if there is no .terraform/environment file, so ignore not exist errors
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	return t.list(rc, currentNamespace, currentWorkspace)
}

func (t *ListWorkspace) list(rc client.Client, currentNamespace, currentWorkspace string) error {
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
		fmt.Fprintf(t.Out, "%s\t%s/%s\n", prefix, ws.GetNamespace(), ws.GetName())
	}

	return nil
}
