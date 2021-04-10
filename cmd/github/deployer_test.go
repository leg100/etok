package github

import (
	"context"
	"errors"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kfake "k8s.io/client-go/kubernetes/fake"

	"github.com/leg100/etok/pkg/scheme"
	"github.com/leg100/etok/pkg/testutil"
	"github.com/leg100/etok/pkg/version"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestDeployer(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		objs       []runtimeclient.Object
		deployer   deployer
		assertions func(*testutil.T, runtimeclient.Client)
	}{
		{
			name:     "default",
			deployer: deployer{},
		},
		{
			name: "update",
			deployer: deployer{
				// Apply is not supported in fake client
				patch: runtimeclient.Merge,
			},
			objs: []runtimeclient.Object{
				newUnstructuredObj(schema.GroupVersionKind{Kind: "ClusterRole", Version: "v1", Group: "rbac.authorization.k8s.io"}, "webhook"),
				newUnstructuredObj(schema.GroupVersionKind{Kind: "ClusterRoleBinding", Version: "v1", Group: "rbac.authorization.k8s.io"}, "webhook"),
				newUnstructuredObj(schema.GroupVersionKind{Kind: "ServiceAccount", Version: "v1"}, "webhook", "github"),
				newUnstructuredObj(schema.GroupVersionKind{Kind: "Deployment", Version: "v1", Group: "apps"}, "webhook", "github"),
			},
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

			client := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(tt.objs...).Build()

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

func TestDeployerWait(t *testing.T) {
	tests := []struct {
		name     string
		objs     []runtime.Object
		deployer deployer
		err      error
	}{
		{
			name: "successful",
			// Seed fake client with already successful deploy
			objs: []runtime.Object{successfulDeploy()},
			deployer: deployer{
				namespace: "github",
				timeout:   100 * time.Millisecond,
				interval:  10 * time.Millisecond,
			},
		},
		{
			name: "failure",
			objs: []runtime.Object{deploy()},
			deployer: deployer{
				namespace: "github",
				timeout:   100 * time.Millisecond,
				interval:  10 * time.Millisecond,
			},
			err: wait.ErrWaitTimeout,
		},
	}
	for _, tt := range tests {
		testutil.Run(t, tt.name, func(t *testutil.T) {
			// Create fake client and seed with any objs
			client := kfake.NewSimpleClientset(tt.objs...)

			err := tt.deployer.wait(context.Background(), client)
			if !assert.True(t, errors.Is(err, tt.err)) {
				t.Errorf("no error in %v's chain matches %v", err, tt.err)
			}
		})
	}
}

func deploy() *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "webhook",
			Namespace: "github",
		},
	}
}

func successfulDeploy() *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "webhook",
			Namespace: "github",
		},
		Status: appsv1.DeploymentStatus{
			Conditions: []appsv1.DeploymentCondition{
				{
					Type:               appsv1.DeploymentAvailable,
					Status:             corev1.ConditionTrue,
					LastTransitionTime: metav1.Time{Time: time.Now().Add(-11 * time.Second)},
				},
			},
		},
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
