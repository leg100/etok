package cmd

import (
	"fmt"
	"time"

	"github.com/leg100/stok/pkg/apis/stok/v1alpha1"
	v1alpha1types "github.com/leg100/stok/pkg/apis/stok/v1alpha1"
	v1alpha1clientset "github.com/leg100/stok/pkg/client/clientset/typed/stok/v1alpha1"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
)

type newWorkspaceCmd struct {
	Name           string
	Namespace      string
	Path           string
	CacheSize      string
	StorageClass   string
	NoSecret       bool
	Secret         string
	ServiceAccount string
	KubeConfigPath string
	Timeout        time.Duration

	cmd *cobra.Command
}

func newNewWorkspaceCmd() *cobra.Command {
	cc := &newWorkspaceCmd{}
	cc.cmd = &cobra.Command{
		Use:   "new <workspace>",
		Short: "Create a new stok workspace",
		Long:  "Deploys a Workspace resource",
		Args:  cobra.ExactArgs(1),
		RunE:  cc.doNewWorkspace,
	}
	cc.cmd.Flags().StringVar(&cc.Path, "path", ".", "workspace config path")
	cc.cmd.Flags().StringVar(&cc.Namespace, "namespace", "default", "Kubernetes namespace of workspace")
	cc.cmd.Flags().StringVar(&cc.ServiceAccount, "service-account", "", "Name of ServiceAccount")
	cc.cmd.Flags().StringVar(&cc.Secret, "secret", "stok", "Name of Secret containing credentials")
	cc.cmd.Flags().BoolVar(&cc.NoSecret, "no-secret", false, "Don't reference a Secret resource")
	cc.cmd.Flags().StringVar(&cc.CacheSize, "size", "1Gi", "Size of PersistentVolume for cache")
	cc.cmd.Flags().StringVar(&cc.StorageClass, "storage-class", "", "StorageClass of PersistentVolume for cache")
	cc.cmd.Flags().StringVar(&cc.KubeConfigPath, "kubeconfig", "", "absolute path to kubeconfig file (default is $HOME/.kube/config)")
	cc.cmd.Flags().DurationVar(&cc.Timeout, "timeout", time.Minute, "Time to wait for workspace to be healthy")

	return cc.cmd
}

// 1. Wait til Workspace resource is healthy
// 2. Run init command
// 3. Write .terraform/environment
func (t *newWorkspaceCmd) doNewWorkspace(cmd *cobra.Command, args []string) error {
	if err := unmarshalV(t); err != nil {
		return err
	}

	t.Name = args[0]

	config, err := configFromPath(t.KubeConfigPath)
	if err != nil {
		return err
	}

	clientset, err := v1alpha1clientset.NewForConfig(config)
	if err != nil {
		return err
	}

	ws, err := t.createWorkspace(clientset)
	if err != nil {
		return err
	}

	fmt.Printf("Created workspace '%s' in namespace '%s'\n", t.Name, t.Namespace)

	_, err = t.waitUntilWorkspaceHealthy(clientset, ws)
	if err != nil {
		return err
	}

	if err := writeEnvironmentFile(t.Path, t.Namespace, t.Name); err != nil {
		return err
	}

	return nil
}

func (t *newWorkspaceCmd) createWorkspace(clientset v1alpha1clientset.StokV1alpha1Interface) (*v1alpha1types.Workspace, error) {
	ws := &v1alpha1types.Workspace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      t.Name,
			Namespace: t.Namespace,
			Labels: map[string]string{
				"app": "stok",
			},
		},
		Spec: v1alpha1types.WorkspaceSpec{},
	}

	if !t.NoSecret {
		ws.Spec.SecretName = t.Secret
	}

	if t.ServiceAccount != "" {
		ws.Spec.ServiceAccountName = t.ServiceAccount
	}

	if t.CacheSize != "" {
		ws.Spec.Cache.Size = t.CacheSize
	}

	if t.StorageClass != "" {
		ws.Spec.Cache.StorageClass = t.StorageClass
	}

	return clientset.Workspaces(t.Namespace).Create(ws)
}

func (t *newWorkspaceCmd) waitUntilWorkspaceHealthy(cs *v1alpha1clientset.StokV1alpha1Client, workspace *v1alpha1types.Workspace) (*v1alpha1types.Workspace, error) {
	obj, err := waitUntil(cs.RESTClient(), workspace, workspace.GetName(), workspace.GetNamespace(), "workspaces", workspaceHealthy, t.Timeout)
	return obj.(*v1alpha1types.Workspace), err
}

// A watchtools.ConditionFunc that returns true when a Workspace resource's WorkspaceHealthy
// condition is true; returns false otherwise, along with an error if WorkspaceHealthy is
// false
func workspaceHealthy(event watch.Event) (bool, error) {
	switch event.Type {
	case watch.Deleted:
		return false, errors.NewNotFound(schema.GroupResource{Resource: "workspaces"}, "")
	}
	switch t := event.Object.(type) {
	case *v1alpha1types.Workspace:
		conditions := t.Status.Conditions
		if conditions == nil {
			return false, nil
		}
		for i := range conditions {
			if conditions[i].Type == v1alpha1.ConditionHealthy {
				if conditions[i].Status == corev1.ConditionTrue {
					return true, nil
				}
				if conditions[i].Status == corev1.ConditionFalse {
					return false, fmt.Errorf(conditions[i].Message)
				}
			}
		}
	}
	return false, nil
}
