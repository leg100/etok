package cmd

import (
	v1alpha1clientset "github.com/leg100/stok/pkg/client/clientset/typed/stok/v1alpha1"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type deleteWorkspaceCmd struct {
	Name           string
	Namespace      string
	Path           string
	KubeConfigPath string

	cmd *cobra.Command
}

func newDeleteWorkspaceCmd() *cobra.Command {
	cc := &deleteWorkspaceCmd{}
	cc.cmd = &cobra.Command{
		Use:     "delete <namespace/workspace>",
		Short:   "Delete a stok workspace",
		Args:    cobra.ExactArgs(1),
		PreRunE: cc.preRun,
		RunE:    cc.doDeleteWorkspace,
	}
	cc.cmd.Flags().StringVar(&cc.Path, "path", ".", "workspace config path")
	cc.cmd.Flags().StringVar(&cc.KubeConfigPath, "kubeconfig", "", "absolute path to kubeconfig file (default is $HOME/.kube/config)")
	cc.cmd.Flags().StringVar(&cc.Namespace, "namespace", "default", "Kubernetes namespace of workspace")

	return cc.cmd
}

func (t *deleteWorkspaceCmd) preRun(cmd *cobra.Command, args []string) error {
	if err := unmarshalV(t); err != nil {
		return err
	}

	t.Name = args[0]

	return nil
}

// wait til Workspace resource is healthy
// write .terraform/environment
func (t *deleteWorkspaceCmd) doDeleteWorkspace(cmd *cobra.Command, args []string) error {
	config, err := configFromPath(t.KubeConfigPath)
	if err != nil {
		return err
	}

	clientset, err := v1alpha1clientset.NewForConfig(config)
	if err != nil {
		return err
	}

	if err = t.deleteWorkspace(clientset); err != nil {
		return err
	}

	return nil
}

func (t *deleteWorkspaceCmd) deleteWorkspace(clientset v1alpha1clientset.StokV1alpha1Interface) error {
	return clientset.Workspaces(t.Namespace).Delete(t.Name, &metav1.DeleteOptions{})
}
