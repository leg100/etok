package github

import (
	"context"
	"testing"

	"github.com/leg100/etok/pkg/scheme"
	"github.com/leg100/etok/pkg/testutil"
	"github.com/leg100/etok/pkg/version"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestDeployer(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		deployer   deployer
		assertions func(*testutil.T, runtimeclient.Client)
	}{
		{
			name:     "default",
			deployer: deployer{},
		},
		{
			name: "image setting",
			deployer: deployer{
				image: "bugsbunny:v123",
			},
			assertions: func(t *testutil.T, client runtimeclient.Client) {
				// Get deployment
				deployment := &unstructured.Unstructured{}
				deployment.SetGroupVersionKind(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"})
				err := client.Get(context.Background(), runtimeclient.ObjectKey{Namespace: "github", Name: "webhook"}, deployment)
				if assert.NoError(t, err) {

					// Get container
					containers, found, err := unstructured.NestedSlice(deployment.Object, "spec", "template", "spec", "containers")
					if assert.True(t, found) && assert.NoError(t, err) && assert.Equal(t, 1, len(containers)) {

						// Get image
						image, found, err := unstructured.NestedString(containers[0].(map[string]interface{}), "image")
						if assert.True(t, found) && assert.NoError(t, err) {
							assert.Equal(t, "bugsbunny:v123", image)
						}
					}
				}
			},
		},
	}
	for _, tt := range tests {
		testutil.Run(t, tt.name, func(t *testutil.T) {
			if tt.deployer.image == "" {
				tt.deployer.image = version.Image
			}

			if tt.deployer.namespace == "" {
				tt.deployer.namespace = defaultNamespace
			}

			if tt.deployer.port == 0 {
				tt.deployer.port = defaultWebhookPort
			}

			client := fake.NewClientBuilder().WithScheme(scheme.Scheme).Build()

			require.NoError(t, tt.deployer.deploy(context.Background(), client))

			// assert resources are present
			wantResources := []*unstructured.Unstructured{
				newUnstructuredObj(schema.GroupVersionKind{Kind: "ClusterRole", Version: "v1", Group: "rbac.authorization.k8s.io"}, "webhook"),
				newUnstructuredObj(schema.GroupVersionKind{Kind: "ClusterRoleBinding", Version: "v1", Group: "rbac.authorization.k8s.io"}, "webhook"),
				newUnstructuredObj(schema.GroupVersionKind{Kind: "ServiceAccount", Version: "v1"}, "webhook", "github"),
				newUnstructuredObj(schema.GroupVersionKind{Kind: "Deployment", Version: "v1", Group: "apps"}, "webhook", "github"),
			}
			for _, res := range wantResources {
				assert.NoError(t, client.Get(context.Background(), runtimeclient.ObjectKeyFromObject(res), res))
			}

			if tt.assertions != nil {
				tt.assertions(t, client)
			}
		})
	}
}

func newUnstructuredObj(gvk schema.GroupVersionKind, name string, namespace ...string) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(gvk)
	u.SetName(name)

	if len(namespace) > 1 {
		panic("only want one namespace")
	}
	if len(namespace) == 1 {
		u.SetNamespace(namespace[0])
	}

	return u
}
