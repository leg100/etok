package cmd

import (
	"fmt"
	"time"

	v1alpha1types "github.com/leg100/stok/pkg/apis/stok/v1alpha1"
	v1alpha1clientset "github.com/leg100/stok/pkg/client/clientset/typed/stok/v1alpha1"
	workspaceController "github.com/leg100/stok/pkg/controller/workspace"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
)

type newWorkspaceCmd struct {
	Name                namespacedWorkspace
	Path                string
	CacheSize           string
	StorageClass        string
	CreateRBACResources bool
	NoSecret            bool
	Secret              string
	ServiceAccount      string
	KubeConfigPath      string
	Timeout             time.Duration

	cmd *cobra.Command
}

func newNewWorkspaceCmd() *cobra.Command {
	cc := &newWorkspaceCmd{}
	cc.cmd = &cobra.Command{
		Use:   "new <namespace/workspace>",
		Short: "Create a new stok workspace",
		Long:  "Deploys a Workspace resource",
		Args: func(cmd *cobra.Command, args []string) error {
			return validateWorkspaceArg(args)
		},
		PreRunE: cc.preRun,
		RunE:    cc.doNewWorkspace,
	}
	cc.cmd.Flags().StringVar(&cc.Path, "path", ".", "workspace config path")
	cc.cmd.Flags().StringVar(&cc.ServiceAccount, "service-account", "stok", "Name of ServiceAccount")
	cc.cmd.Flags().StringVar(&cc.Secret, "secret", "stok", "Name of Secret containing credentials")
	cc.cmd.Flags().BoolVar(&cc.NoSecret, "no-secret", false, "Don't reference a Secret resource")
	cc.cmd.Flags().BoolVar(&cc.CreateRBACResources, "create-rbac-resources", false, "Create RBAC resources: ServiceAccount, Role, and RoleBinding")
	cc.cmd.Flags().StringVar(&cc.CacheSize, "size", "1Gi", "Size of PersistentVolume for cache")
	cc.cmd.Flags().StringVar(&cc.StorageClass, "storage-class", "", "StorageClass of PersistentVolume for cache")
	cc.cmd.Flags().StringVar(&cc.KubeConfigPath, "kubeconfig", "", "absolute path to kubeconfig file (default is $HOME/.kube/config)")
	cc.cmd.Flags().DurationVar(&cc.Timeout, "timeout", time.Minute, "Time to wait for workspace to be healthy")

	return cc.cmd
}

func (t *newWorkspaceCmd) preRun(cmd *cobra.Command, args []string) error {
	if err := unmarshalV(t); err != nil {
		return err
	}

	t.Name = namespacedWorkspace(args[0])

	return nil
}

// wait til Workspace resource is healthy
// write .terraform/environment
func (t *newWorkspaceCmd) doNewWorkspace(cmd *cobra.Command, args []string) error {
	config, err := configFromPath(t.KubeConfigPath)
	if err != nil {
		return err
	}

	clientset, err := v1alpha1clientset.NewForConfig(config)
	if err != nil {
		return err
	}

	if t.CreateRBACResources {
		client, err := kubernetes.NewForConfig(config)
		if err != nil {
			return err
		}

		if _, err := t.createServiceAccount(client); err != nil {
			return err
		}
		if _, err := t.createRole(client); err != nil {
			return err
		}
		if _, err := t.createRoleBinding(client); err != nil {
			return err
		}
	}

	ws, err := t.createWorkspace(clientset)
	if err != nil {
		return err
	}

	ws, err = t.waitUntilWorkspaceHealthy(clientset, ws)
	if err != nil {
		return err
	}

	if err := t.Name.writeEnvironmentFile(t.Path); err != nil {
		return err
	}

	return nil
}

func (t *newWorkspaceCmd) createServiceAccount(client kubernetes.Interface) (*corev1.ServiceAccount, error) {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      t.ServiceAccount,
			Namespace: t.Name.getNamespace(),
			Labels: map[string]string{
				"app": "stok",
			},
		},
	}

	return client.CoreV1().ServiceAccounts(t.Name.getNamespace()).Create(sa)
}

func (t *newWorkspaceCmd) createRole(client kubernetes.Interface) (*rbacv1.Role, error) {
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "stok",
			Namespace: t.Name.getNamespace(),
			Labels: map[string]string{
				"app": "stok",
			},
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"stok.goalspike.com"},
				Resources: []string{"*"},
				Verbs:     []string{"get", "list", "watch"},
			},
		},
	}

	return client.RbacV1().Roles(t.Name.getNamespace()).Create(role)
}

func (t *newWorkspaceCmd) createRoleBinding(client kubernetes.Interface) (*rbacv1.RoleBinding, error) {
	rb := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "stok",
			Namespace: t.Name.getNamespace(),
			Labels: map[string]string{
				"app": "stok",
			},
		},
		Subjects: []rbacv1.Subject{
			{
				Kind: "ServiceAccount",
				Name: t.ServiceAccount,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "Role",
			Name:     "stok",
			APIGroup: "rbac.authorization.k8s.io",
		},
	}

	return client.RbacV1().RoleBindings(t.Name.getNamespace()).Create(rb)
}

func (t *newWorkspaceCmd) createWorkspace(clientset v1alpha1clientset.StokV1alpha1Interface) (*v1alpha1types.Workspace, error) {
	ws := &v1alpha1types.Workspace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      t.Name.getWorkspace(),
			Namespace: t.Name.getNamespace(),
			Labels: map[string]string{
				"app": "stok",
			},
		},
		Spec: v1alpha1types.WorkspaceSpec{
			ServiceAccountName: t.ServiceAccount,
		},
	}

	if !t.NoSecret {
		ws.Spec.SecretName = t.Secret
	}

	if t.CacheSize != "" {
		ws.Spec.Cache.Size = t.CacheSize
	}

	if t.StorageClass != "" {
		ws.Spec.Cache.StorageClass = t.StorageClass
	}

	return clientset.Workspaces(t.Name.getNamespace()).Create(ws)
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
			if conditions[i].Type == workspaceController.WorkspaceHealthy {
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
