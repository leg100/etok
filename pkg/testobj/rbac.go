package testobj

import (
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Role(namespace, name string, opts ...func(*rbacv1.Role)) *rbacv1.Role {
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}

	for _, option := range opts {
		option(role)
	}

	return role
}

func WithRule(rule rbacv1.PolicyRule) func(role *rbacv1.Role) {
	return func(role *rbacv1.Role) {
		role.Rules = append(role.Rules, rule)
	}
}
