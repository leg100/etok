package cmd

import (
	"fmt"
	"io"
	"os"

	v1alpha1clientset "github.com/leg100/stok/pkg/client/clientset/typed/stok/v1alpha1"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type listWorkspaceCmd struct {
	Path           string
	KubeConfigPath string

	cmd *cobra.Command
}

func newListWorkspaceCmd() *cobra.Command {
	cc := &listWorkspaceCmd{}
	cc.cmd = &cobra.Command{
		Use:   "list",
		Short: "List all workspaces",
		RunE:  cc.doListWorkspace,
	}
	cc.cmd.Flags().StringVar(&cc.Path, "path", ".", "workspace config path")
	cc.cmd.Flags().StringVar(&cc.KubeConfigPath, "kubeconfig", "", "absolute path to kubeconfig file (default is $HOME/.kube/config)")

	return cc.cmd
}

func (t *listWorkspaceCmd) doListWorkspace(cmd *cobra.Command, args []string) error {
	config, err := configFromPath(t.KubeConfigPath)
	if err != nil {
		return err
	}

	clientset, err := v1alpha1clientset.NewForConfig(config)
	if err != nil {
		return err
	}

	current, err := readEnvironmentFile(t.Path)
	if err != nil {
		return err
	}

	err = t.listWorkspaces(clientset, current.getWorkspace(), os.Stdout)
	if err != nil {
		return err
	}

	return nil
}

func (t *listWorkspaceCmd) listWorkspaces(clientset v1alpha1clientset.StokV1alpha1Interface, current string, writer io.Writer) error {
	workspaces, err := clientset.Workspaces("").List(metav1.ListOptions{})
	if err != nil {
		return err
	}

	var prefix string
	for _, ws := range workspaces.Items {
		if ws.GetName() == current {
			prefix = "*"
		} else {
			prefix = ""
		}
		fmt.Fprintf(writer, "%s\t%s\n", prefix, ws.GetName())
	}

	return nil
}
