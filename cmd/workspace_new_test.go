package cmd

import (
	"testing"

	"github.com/leg100/stok/pkg/apis"
	"k8s.io/kubectl/pkg/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

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
			name:      "no-service-account",
			namespace: "default",
			noSecret:  true,
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
				Name:           tt.name,
				Namespace:      tt.namespace,
				ServiceAccount: tt.serviceAccount,
				Secret:         tt.secret,
				StorageClass:   tt.storageClass,
				CacheSize:      tt.cacheSize,
			}

			s := scheme.Scheme
			// adds CRD GVKs
			apis.AddToScheme(s)

			client := fake.NewFakeClientWithScheme(s)

			ws, err := nwc.createWorkspace(client)
			if err != nil {
				t.Fatal(err)
			}

			if ws.GetName() != tt.name {
				t.Errorf("want %s got %s", tt.name, ws.GetName())
			}

			if ws.GetNamespace() != tt.namespace {
				t.Errorf("want %s got %s", tt.namespace, ws.GetNamespace())
			}

			if tt.serviceAccount != "" {
				if ws.Spec.ServiceAccountName != tt.serviceAccount {
					t.Errorf("want %s got %s", tt.serviceAccount, ws.Spec.ServiceAccountName)
				}
			} else {
				if ws.Spec.ServiceAccountName != "" {
					t.Errorf("want '' got %s", ws.Spec.ServiceAccountName)
				}
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
