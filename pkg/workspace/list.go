package workspace

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/leg100/stok/pkg/k8s"
	"github.com/leg100/stok/pkg/k8s/stokclient"
	"github.com/leg100/stok/util"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ListWorkspace struct {
	Path    string
	Context string

	Out io.Writer
	Cmd *cobra.Command
}

func (t *ListWorkspace) Run(ctx context.Context) error {
	sc, err := k8s.StokClient()
	if err != nil {
		return err
	}

	currentNamespace, currentWorkspace, err := util.ReadEnvironmentFile(t.Path)
	// It's ok if there is no .terraform/environment file, so ignore not exist errors
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	return t.list(ctx, sc, currentNamespace, currentWorkspace)
}

func (t *ListWorkspace) list(ctx context.Context, sc stokclient.Interface, currentNamespace, currentWorkspace string) error {
	// List across all namespaces
	workspaces, err := sc.StokV1alpha1().Workspaces("").List(ctx, metav1.ListOptions{})
	if err != nil {
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
