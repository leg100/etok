package cmd

import (
	"context"
	"flag"

	"github.com/apex/log"
	"github.com/leg100/stok/api/v1alpha1"
	"github.com/leg100/stok/pkg/k8s"
	"github.com/leg100/stok/scheme"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type deleteWorkspaceCmd struct {
	Name      string
	Namespace string
	Path      string
	Context   string

	factory k8s.FactoryInterface
	cmd     *cobra.Command
}

func newDeleteWorkspaceCmd(f k8s.FactoryInterface) *cobra.Command {
	cc := &deleteWorkspaceCmd{}
	cc.cmd = &cobra.Command{
		Use:   "delete <namespace/workspace>",
		Short: "Delete a stok workspace",
		Args:  cobra.ExactArgs(1),
		RunE:  cc.doDeleteWorkspace,
	}
	cc.cmd.Flags().StringVar(&cc.Path, "path", ".", "workspace config path")
	cc.cmd.Flags().StringVar(&cc.Namespace, "namespace", "default", "Kubernetes namespace of workspace")
	cc.cmd.Flags().StringVar(&cc.Context, "context", "", "Set kube context (defaults to kubeconfig current context)")

	// Add flags registered by imported packages (controller-runtime)
	cc.cmd.Flags().AddGoFlagSet(flag.CommandLine)

	cc.factory = f

	return cc.cmd
}

// wait til Workspace resource is healthy
// write .terraform/environment
func (t *deleteWorkspaceCmd) doDeleteWorkspace(cmd *cobra.Command, args []string) error {
	if err := unmarshalV(t); err != nil {
		return err
	}

	t.Name = args[0]

	// Controller-runtime client for constructing workspace resource
	rc, err := t.factory.NewClient(scheme.Scheme, t.Context)
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
	return rc.Delete(context.TODO(), &v1alpha1.Workspace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      t.Name,
			Namespace: t.Namespace,
		},
	})
}
