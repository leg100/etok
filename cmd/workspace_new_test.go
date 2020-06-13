package cmd

import (
	"testing"

	fakeStokClient "github.com/leg100/stok/pkg/client/clientset/fake"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestCreateServiceAccount(t *testing.T) {
	nwc := &newWorkspaceCmd{
		Name:           "default/default",
		ServiceAccount: "stok",
	}

	sa := &corev1.ServiceAccount{}
	clientset := fake.NewSimpleClientset()

	sa, err := nwc.createServiceAccount(clientset)
	if err != nil {
		t.Fatal(err)
	}

	if sa.GetName() != "stok" {
		t.Errorf("want 'stok' got %s", sa.GetName())
	}

	if sa.GetNamespace() != "default" {
		t.Errorf("want 'default' got %s", sa.GetName())
	}
}

func TestCreateRole(t *testing.T) {
	nwc := &newWorkspaceCmd{
		Name: "default/default",
	}

	role := &rbacv1.Role{}
	clientset := fake.NewSimpleClientset()

	role, err := nwc.createRole(clientset)
	if err != nil {
		t.Fatal(err)
	}

	if role.GetName() != "stok" {
		t.Errorf("want 'stok' got %s", role.GetName())
	}

	if role.GetNamespace() != "default" {
		t.Errorf("want 'default' got %s", role.GetName())
	}
}

func TestCreateRoleBinding(t *testing.T) {
	nwc := &newWorkspaceCmd{
		Name:           "default/default",
		ServiceAccount: "my-service-account",
	}

	rolebinding := &rbacv1.RoleBinding{}
	clientset := fake.NewSimpleClientset()

	rolebinding, err := nwc.createRoleBinding(clientset)
	if err != nil {
		t.Fatal(err)
	}

	if rolebinding.GetName() != "stok" {
		t.Errorf("want 'stok' got %s", rolebinding.GetName())
	}

	if rolebinding.GetNamespace() != "default" {
		t.Errorf("want 'default' got %s", rolebinding.GetName())
	}

	if rolebinding.Subjects[0].Name != "my-service-account" {
		t.Errorf("want 'my-service-account' got %s", rolebinding.Subjects[0].Name)
	}
}

func TestCreateWorkspace(t *testing.T) {
	tests := []struct {
		name           string
		namespace      string
		serviceAccount string
		secret         string
		noSecret       bool
		storageClass   string
		cacheSize      string
	}{
		{
			name:           "service-account-and-secret",
			namespace:      "default",
			serviceAccount: "stok",
			secret:         "stok",
		},
		{
			name:           "no-secret",
			namespace:      "default",
			serviceAccount: "stok",
			noSecret:       true,
		},
		{
			name:           "storage-class",
			namespace:      "default",
			serviceAccount: "stok",
			noSecret:       true,
			storageClass:   "a-storage-class",
		},
		{
			name:           "cache-size",
			namespace:      "default",
			serviceAccount: "stok",
			noSecret:       true,
			cacheSize:      "3Gi",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nwc := &newWorkspaceCmd{
				Name:           newNamespacedWorkspace(tt.namespace, tt.name),
				ServiceAccount: tt.serviceAccount,
				Secret:         tt.secret,
				StorageClass:   tt.storageClass,
				CacheSize:      tt.cacheSize,
			}

			clientset := fakeStokClient.NewSimpleClientset()

			ws, err := nwc.createWorkspace(clientset.StokV1alpha1())
			if err != nil {
				t.Fatal(err)
			}

			if ws.GetName() != tt.name {
				t.Errorf("want %s got %s", tt.name, ws.GetName())
			}

			if ws.GetNamespace() != tt.namespace {
				t.Errorf("want %s got %s", tt.namespace, ws.GetNamespace())
			}

			if ws.Spec.ServiceAccountName != tt.serviceAccount {
				t.Errorf("want %s got %s", tt.serviceAccount, ws.Spec.ServiceAccountName)
			}

			if tt.noSecret {
				if ws.Spec.SecretName != "" {
					t.Errorf("want '' got %s", ws.Spec.SecretName)
				}
			} else {
				if ws.Spec.SecretName != tt.secret {
					t.Errorf("want %s got %s", tt.secret, ws.Spec.SecretName)
				}
			}

			if ws.Spec.Cache.StorageClass != tt.storageClass {
				t.Errorf("want %s got %s", tt.storageClass, ws.Spec.Cache.StorageClass)
			}

			if ws.Spec.Cache.Size != tt.cacheSize {
				t.Errorf("want %s got %s", tt.cacheSize, ws.Spec.Cache.Size)
			}
		})
	}
}
