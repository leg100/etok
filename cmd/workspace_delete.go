package cmd

import (
	"context"

	"github.com/apex/log"
	"github.com/leg100/stok/pkg/apis"
	"github.com/leg100/stok/pkg/apis/stok/v1alpha1"
	"github.com/spf13/cobra"
	"k8s.io/kubectl/pkg/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
		Use:   "delete <namespace/workspace>",
		Short: "Delete a stok workspace",
		Args:  cobra.ExactArgs(1),
		RunE:  cc.doDeleteWorkspace,
	}
	cc.cmd.Flags().StringVar(&cc.Path, "path", ".", "workspace config path")
	cc.cmd.Flags().StringVar(&cc.KubeConfigPath, "kubeconfig", "", "absolute path to kubeconfig file (default is $HOME/.kube/config)")
	cc.cmd.Flags().StringVar(&cc.Namespace, "namespace", "default", "Kubernetes namespace of workspace")

	return cc.cmd
}

// wait til Workspace resource is healthy
// write .terraform/environment
func (t *deleteWorkspaceCmd) doDeleteWorkspace(cmd *cobra.Command, args []string) error {
	if err := unmarshalV(t); err != nil {
		return err
	}

	t.Name = args[0]

	config, err := configFromPath(t.KubeConfigPath)
	if err != nil {
		return err
	}

	// Get built-in scheme
	s := scheme.Scheme
	// And add our CRDs
	apis.AddToScheme(s)

	// Controller-runtime client for constructing workspace resource
	rc, err := client.New(config, client.Options{Scheme: s})
	if err != nil {
		return err
	}

	if err = t.deleteWorkspace(rc); err != nil {
		return err
	}

	log.WithFields(log.Fields{
		"workspace": t.Name,
		"namespace": t.Namespace,
	}).Info("Deleted workspace")

	return nil
}

func (t *deleteWorkspaceCmd) deleteWorkspace(rc client.Client) error {
	ws := v1alpha1.Workspace{}
	ws.SetName(t.Name)
	ws.SetNamespace(t.Namespace)
	return rc.Delete(context.TODO(), &ws)
}
