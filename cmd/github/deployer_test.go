package github

import (
	"context"
	"testing"

	"github.com/leg100/etok/pkg/scheme"
	"github.com/leg100/etok/pkg/testutil"
	"github.com/leg100/etok/pkg/version"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/types"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestDeployer(t *testing.T) {
	tests := []struct {
		name       string
		objs       []runtimeclient.Object
		err        error
		deployer   deployer
		assertions func(*testutil.T, runtimeclient.Client)
	}{
		{
			name: "install: deployment image",
			assertions: func(t *testutil.T, client runtimeclient.Client) {
				var deploy appsv1.Deployment
				client.Get(context.Background(), types.NamespacedName{Namespace: defaultNamespace, Name: "webhook"}, &deploy)
				assert.Equal(t, version.Image, deploy.Spec.Template.Spec.Containers[0].Image)
			},
		},
		{
			name: "update: deployment image",
			deployer: deployer{
				image: "bugsbunny:v123",
			},
			objs: []runtimeclient.Object{deployment(defaultNamespace, version.Image, defaultWebhookPort)},
			assertions: func(t *testutil.T, client runtimeclient.Client) {
				var deploy appsv1.Deployment
				client.Get(context.Background(), types.NamespacedName{Namespace: defaultNamespace, Name: "webhook"}, &deploy)
				assert.Equal(t, "bugsbunny:v123", deploy.Spec.Template.Spec.Containers[0].Image)
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

			client := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(tt.objs...).Build()

			require.NoError(t, tt.deployer.deploy(context.Background(), client))

			if tt.assertions != nil {
				tt.assertions(t, client)
			}
		})
	}
}
