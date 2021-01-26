package controllers

import (
	v1alpha1 "github.com/leg100/etok/api/etok.dev/v1alpha1"
	"github.com/leg100/etok/pkg/labels"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func newVariablesForWS(ws *v1alpha1.Workspace) *corev1.ConfigMap {
	variables := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ws.VariablesConfigMapName(),
			Namespace: ws.Namespace,
		},
		Data: map[string]string{
			variablesPath: `variable "namespace" {}
variable "workspace" {}
`,
		},
	}

	// Set etok's common labels
	labels.SetCommonLabels(variables)
	// Permit filtering etok resources by component
	labels.SetLabel(variables, labels.WorkspaceComponent)

	return variables
}

func newPVCForWS(ws *v1alpha1.Workspace) *corev1.PersistentVolumeClaim {
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ws.Name,
			Namespace: ws.Namespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse(ws.Spec.Cache.Size),
				},
			},
			StorageClassName: ws.Spec.Cache.StorageClass,
		},
	}

	// Set etok's common labels
	labels.SetCommonLabels(pvc)
	// Permit filtering etok resources by component
	labels.SetLabel(pvc, labels.WorkspaceComponent)

	return pvc
}

func newRoleForNamespace(ws *v1alpha1.Workspace) *rbacv1.Role {
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ws.Namespace,
			Name:      RoleName,
		},
		Rules: []rbacv1.PolicyRule{
			// Runner may need to persist a lock file to a new config map
			{
				Resources: []string{"configmaps"},
				Verbs:     []string{"create"},
				APIGroups: []string{""},
			},
			// ...and the runner specifies the run resource as owner of said
			// config map, so it needs to retrieve run resource metadata as well
			{
				Resources: []string{"runs"},
				Verbs:     []string{"get"},
				APIGroups: []string{"etok.dev"},
			},
			// Terraform state backend mgmt
			{
				Resources: []string{"secrets"},
				Verbs:     []string{"list", "create", "get", "delete", "patch", "update"},
				APIGroups: []string{""},
			},
			// Terraform state backend mgmt
			{
				Resources: []string{"leases"},
				Verbs:     []string{"list", "create", "get", "delete", "patch", "update"},
				APIGroups: []string{"coordination.k8s.io"},
			},
		},
	}

	// Set etok's common labels
	labels.SetCommonLabels(role)
	// Permit filtering etok resources by component
	labels.SetLabel(role, labels.WorkspaceComponent)

	return role
}

func newRoleBindingForNamespace(ws *v1alpha1.Workspace) *rbacv1.RoleBinding {
	binding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ws.Namespace,
			Name:      RoleBindingName,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      ServiceAccountName,
				Namespace: ws.Namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "Role",
			Name:     RoleName,
			APIGroup: "rbac.authorization.k8s.io",
		},
	}

	// Set etok's common labels
	labels.SetCommonLabels(binding)
	// Permit filtering etok resources by component
	labels.SetLabel(binding, labels.WorkspaceComponent)

	return binding
}

func newServiceAccountForNamespace(ws *v1alpha1.Workspace) *corev1.ServiceAccount {
	serviceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ws.Namespace,
			Name:      ServiceAccountName,
		},
	}

	// Set etok's common labels
	labels.SetCommonLabels(serviceAccount)
	// Permit filtering etok resources by component
	labels.SetLabel(serviceAccount, labels.WorkspaceComponent)

	return serviceAccount
}
