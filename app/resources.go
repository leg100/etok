package app

import (
	"bytes"
	"context"

	"github.com/leg100/stok/pkg/apis/terraform/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (app *App) CreateRole(name string) (string, error) {
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: app.Namespace,
			Labels: map[string]string{
				"app": "stok",
			},
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups:     []string{"stok.goalspike.com"},
				Resources:     []string{"commands"},
				ResourceNames: []string{name},
				Verbs:         []string{"list", "get", "watch"},
			},
		},
	}

	role, err := app.KubeClient.RbacV1().Roles(app.Namespace).Create(role)
	if err != nil {
		return "", err
	}
	app.AddToCleanup(role)

	app.Logger.Debugw("resource created", "type", "role", "name", role.GetName(), "namespace", app.Namespace)

	return role.GetName(), nil
}

func (app *App) CreateServiceAccount(name string) (string, error) {
	serviceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}

	serviceAccount, err := app.KubeClient.CoreV1().ServiceAccounts(app.Namespace).Create(serviceAccount)
	if err != nil {
		return "", err
	}
	app.AddToCleanup(serviceAccount)

	app.Logger.Debugw("resource created", "type", "serviceAccount", "name", serviceAccount.GetName(), "namespace", app.Namespace)

	return serviceAccount.GetName(), nil
}

func (app *App) CreateRoleBinding(name string) (string, error) {
	binding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: app.Namespace,
			Labels: map[string]string{
				"app": "stok",
			},
		},
		Subjects: []rbacv1.Subject{
			{
				Name: name,
				Kind: "ServiceAccount",
			},
		},
		RoleRef: rbacv1.RoleRef{
			Name:     name,
			Kind:     "Role",
			APIGroup: "rbac.authorization.k8s.io",
		},
	}

	binding, err := app.KubeClient.RbacV1().RoleBindings(app.Namespace).Create(binding)
	if err != nil {
		return "", err
	}
	app.AddToCleanup(binding)

	app.Logger.Debugw("resource created", "type", "rolebinding", "name", binding.GetName(), "namespace", app.Namespace)

	return binding.GetName(), nil
}

func (app *App) CreateConfigMap(tarball *bytes.Buffer) (string, error) {
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "stok-",
			Namespace:    app.Namespace,
			Labels: map[string]string{
				"app": "stok",
			},
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		BinaryData: map[string][]byte{
			// TODO: use constant
			"tarball.tar.gz": tarball.Bytes(),
		},
	}

	configMap, err := app.KubeClient.CoreV1().ConfigMaps(app.Namespace).Create(configMap)
	if err != nil {
		return "", err
	}
	app.AddToCleanup(configMap)

	app.Logger.Debugw("resource created", "type", "configmap", "name", configMap.GetName(), "namespace", app.Namespace)

	return configMap.GetName(), nil
}

func (app *App) CreateCommand(name string) (*v1alpha1.Command, error) {
	command := &v1alpha1.Command{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: app.Namespace,
			Labels: map[string]string{
				"workspace": app.Workspace,
			},
		},
		Spec: v1alpha1.CommandSpec{
			Command:   app.Command,
			Args:      app.Args,
			ConfigMap: name,
			// TODO: use constant
			ConfigMapKey: "tarball.tar.gz",
		},
	}

	if err := app.Client.Create(context.Background(), command); err != nil {
		return command, err
	}
	app.AddToCleanup(command)

	app.Logger.Debugw("resource created", "type", "command", "name", command.GetName(), "namespace", app.Namespace)

	return command, nil
}
