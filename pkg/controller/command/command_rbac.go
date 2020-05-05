package command

import (
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// Service Account for the Command Pod
type CommandServiceAccount struct {
	corev1.ServiceAccount
}

func (sa *CommandServiceAccount) GetRuntimeObj() runtime.Object {
	return &sa.ServiceAccount
}

func (sa *CommandServiceAccount) Construct() error {
	return nil
}

// Role giving permissions to watch the Command resource
type CommandRole struct {
	rbacv1.Role
	commandName string
}

func (r *CommandRole) GetRuntimeObj() runtime.Object {
	return &r.Role
}

func (r *CommandRole) Construct() error {
	r.Rules = []rbacv1.PolicyRule{
		{
			APIGroups:     []string{"stok.goalspike.com"},
			Resources:     []string{"commands"},
			ResourceNames: []string{r.commandName},
			Verbs:         []string{"list", "get", "watch"},
		},
	}
	return nil
}

// RoleBinding binding the above Role to the above ServiceAccount
type CommandRoleBinding struct {
	rbacv1.RoleBinding
	serviceAccountName string
	roleName           string
}

func (rb *CommandRoleBinding) GetRuntimeObj() runtime.Object {
	return &rb.RoleBinding
}

func (rb *CommandRoleBinding) Construct() error {
	rb.Subjects = []rbacv1.Subject{
		{
			Name: rb.serviceAccountName,
			Kind: "ServiceAccount",
		},
	}
	rb.RoleRef = rbacv1.RoleRef{
		Name:     rb.roleName,
		Kind:     "Role",
		APIGroup: "rbac.authorization.k8s.io",
	}
	return nil
}
