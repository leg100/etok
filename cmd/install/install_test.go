package install

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	kfake "k8s.io/client-go/kubernetes/fake"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/leg100/etok/cmd/backup"
	cmdutil "github.com/leg100/etok/cmd/util"
	"github.com/leg100/etok/pkg/client"
	"github.com/leg100/etok/pkg/testobj"
	"github.com/leg100/etok/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
)

func TestInstall(t *testing.T) {
	tests := []struct {
		name             string
		args             []string
		objs             []runtimeclient.Object
		err              error
		assertions       func(*testutil.T, runtimeclient.Client)
		dryRunAssertions func(*testutil.T, *bytes.Buffer)
	}{
		{
			name: "fresh install",
			args: []string{"install", "--wait=false"},
		},
		{
			name: "fresh install with only CRDs",
			args: []string{"install", "--wait=false", "--crds-only"},
		},
		{
			name: "fresh install with service account annotations",
			args: []string{"install", "--wait=false", "--sa-annotations", "foo=bar,baz=haj"},
			assertions: func(t *testutil.T, client runtimeclient.Client) {
				sa := &unstructured.Unstructured{}
				sa.SetGroupVersionKind(schema.GroupVersionKind{Version: "v1", Kind: "ServiceAccount"})
				err := client.Get(context.Background(), runtimeclient.ObjectKey{Namespace: "etok", Name: "etok"}, sa)
				if assert.NoError(t, err) {
					assert.Equal(t, map[string]string{"foo": "bar", "baz": "haj"}, sa.GetAnnotations())
				}
			},
		},
		{
			name: "fresh install with custom image",
			args: []string{"install", "--wait=false", "--image", "bugsbunny:v123"},
			assertions: func(t *testutil.T, client runtimeclient.Client) {
				// Get deployment
				deployment := &unstructured.Unstructured{}
				deployment.SetGroupVersionKind(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"})
				err := client.Get(context.Background(), runtimeclient.ObjectKey{Namespace: "etok", Name: "etok"}, deployment)
				if assert.NoError(t, err) {

					// Get container
					containers, found, err := unstructured.NestedSlice(deployment.Object, "spec", "template", "spec", "containers")
					if assert.True(t, found) && assert.NoError(t, err) && assert.Equal(t, 1, len(containers)) {

						// Get image
						image, found, err := unstructured.NestedString(containers[0].(map[string]interface{}), "image")
						if assert.True(t, found) && assert.NoError(t, err) {
							assert.Equal(t, "bugsbunny:v123", image)
						}

						// Get envs - check that ETOK_IMAGE is set too
						env, found, err := unstructured.NestedSlice(containers[0].(map[string]interface{}), "env")
						if assert.True(t, found) && assert.NoError(t, err) {

							var foundEnvVar bool
							for i := range env {
								name, found, err := unstructured.NestedFieldNoCopy(env[i].(map[string]interface{}), "name")
								require.NoError(t, err)
								if found && name == "ETOK_IMAGE" {

									value, found, err := unstructured.NestedFieldNoCopy(env[i].(map[string]interface{}), "value")
									require.NoError(t, err)
									if found && value == "bugsbunny:v123" {
										foundEnvVar = true
									}
								}
							}
							assert.True(t, foundEnvVar)
						}
					}
				}
			},
		},
		{
			name: "fresh install with secret found",
			args: []string{"install", "--wait=false"},
			objs: []runtimeclient.Object{testobj.Secret("etok", "etok")},
			assertions: func(t *testutil.T, client runtimeclient.Client) {
				// Get deployment
				deployment := &unstructured.Unstructured{}
				deployment.SetGroupVersionKind(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"})
				err := client.Get(context.Background(), runtimeclient.ObjectKey{Namespace: "etok", Name: "etok"}, deployment)

				if assert.NoError(t, err) {

					// Get container
					containers, found, err := unstructured.NestedSlice(deployment.Object, "spec", "template", "spec", "containers")
					if assert.True(t, found) && assert.NoError(t, err) && assert.Equal(t, 1, len(containers)) {

						// Get envFrom
						envfrom, found, err := unstructured.NestedSlice(containers[0].(map[string]interface{}), "envFrom")
						if assert.True(t, found) && assert.NoError(t, err) && assert.Equal(t, 1, len(envfrom)) {

							// Get secretRef
							secretRef, found, err := unstructured.NestedStringMap(envfrom[0].(map[string]interface{}), "secretRef")
							if assert.True(t, found) && assert.NoError(t, err) {
								assert.Equal(t, map[string]string{"name": "etok"}, secretRef)
							}
						}

					}
				}
			},
		},
		{
			name: "fresh install with backups enabled",
			args: []string{"install", "--wait=false", "--backup-provider=gcs", "--gcs-bucket=backups-bucket"},
			assertions: func(t *testutil.T, client runtimeclient.Client) {
				// Get deployment
				deployment := &unstructured.Unstructured{}
				deployment.SetGroupVersionKind(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"})
				err := client.Get(context.Background(), runtimeclient.ObjectKey{Namespace: "etok", Name: "etok"}, deployment)

				if assert.NoError(t, err) {

					// Get container
					containers, found, err := unstructured.NestedSlice(deployment.Object, "spec", "template", "spec", "containers")
					if assert.True(t, found) && assert.NoError(t, err) && assert.Equal(t, 1, len(containers)) {

						// Get envs
						env, found, err := unstructured.NestedSlice(containers[0].(map[string]interface{}), "env")
						if assert.True(t, found) && assert.NoError(t, err) {

							// Get provider env var
							var foundProvider bool
							for i := range env {
								name, found, err := unstructured.NestedFieldNoCopy(env[i].(map[string]interface{}), "name")
								require.NoError(t, err)
								if found && name == "ETOK_BACKUP_PROVIDER" {

									value, found, err := unstructured.NestedFieldNoCopy(env[i].(map[string]interface{}), "value")
									require.NoError(t, err)
									if found && value == "gcs" {
										foundProvider = true
									}
								}
							}
							assert.True(t, foundProvider)

							// Get bucket env var
							var foundBucket bool
							for i := range env {
								name, found, err := unstructured.NestedFieldNoCopy(env[i].(map[string]interface{}), "name")
								require.NoError(t, err)
								if found && name == "ETOK_GCS_BUCKET" {

									value, found, err := unstructured.NestedFieldNoCopy(env[i].(map[string]interface{}), "value")
									require.NoError(t, err)
									if found && value == "backups-bucket" {
										foundBucket = true
									}
								}
							}
							assert.True(t, foundBucket)
						}
					}
				}
			},
		},
		{
			name: "missing backup bucket name",
			args: []string{"install", "--wait=false", "--backup-provider=gcs"},
			err:  backup.ErrInvalidConfig,
		},
		{
			name: "invalid backup provider name",
			args: []string{"install", "--wait=false", "--backup-provider=alibaba-cloud-blob"},
			err:  backup.ErrInvalidConfig,
		},
		{
			name: "dry run",
			args: []string{"install", "--dry-run"},
			dryRunAssertions: func(t *testutil.T, out *bytes.Buffer) {
				// Assert correct number of k8s objs are serialized to yaml
				docs := strings.Split(out.String(), "---\n")
				assert.Equal(t, 8, len(docs))
			},
		},
	}
	for _, tt := range tests {
		testutil.Run(t, tt.name, func(t *testutil.T) {
			buf := new(bytes.Buffer)

			f := &cmdutil.Factory{
				IOStreams:            cmdutil.IOStreams{Out: buf},
				ClientCreator:        client.NewFakeClientCreator(),
				RuntimeClientCreator: NewFakeClientCreator(convertObjs(tt.objs...)...),
			}

			cmd, opts := InstallCmd(f)
			cmd.SetOut(buf)
			cmd.SetArgs(tt.args)

			// Run command and assert returned error is either nil or wraps
			// expected error
			err := cmd.ExecuteContext(context.Background())
			if !assert.True(t, errors.Is(err, tt.err)) {
				t.Errorf("unexpected error: %w", err)
				t.FailNow()
			}
			if err != nil {
				// Expected error occurred; there's no point in continuing
				return
			}

			// Perform dry run assertions and skip k8s tests
			if tt.dryRunAssertions != nil {
				tt.dryRunAssertions(t, buf)
				return
			}

			wantResources := []*unstructured.Unstructured{
				newUnstructuredObj(schema.GroupVersionKind{Kind: "ClusterRole", Version: "v1", Group: "rbac.authorization.k8s.io"}, "etok"),
				newUnstructuredObj(schema.GroupVersionKind{Kind: "ClusterRole", Version: "v1", Group: "rbac.authorization.k8s.io"}, "etok-admin"),
				newUnstructuredObj(schema.GroupVersionKind{Kind: "ClusterRole", Version: "v1", Group: "rbac.authorization.k8s.io"}, "etok-user"),
				newUnstructuredObj(schema.GroupVersionKind{Kind: "ClusterRoleBinding", Version: "v1", Group: "rbac.authorization.k8s.io"}, "etok"),
				newUnstructuredObj(schema.GroupVersionKind{Kind: "ServiceAccount", Version: "v1"}, "etok", "etok"),
				newUnstructuredObj(schema.GroupVersionKind{Kind: "Deployment", Version: "v1", Group: "apps"}, "etok", "etok"),
			}

			// assert non-CRD resources are present
			if !opts.crdsOnly {
				for _, res := range wantResources {
					assert.NoError(t, opts.RuntimeClient.Get(context.Background(), runtimeclient.ObjectKeyFromObject(res), res))
				}
			}

			// assert CRDs are present
			crdGVK := schema.GroupVersionKind{Kind: "CustomResourceDefinition", Group: "apiextensions.k8s.io", Version: "v1"}
			wantCRDs := []*unstructured.Unstructured{
				newUnstructuredObj(crdGVK, "workspaces.etok.dev"),
				newUnstructuredObj(crdGVK, "runs.etok.dev"),
			}
			for _, crd := range wantCRDs {
				assert.NoError(t, opts.RuntimeClient.Get(context.Background(), runtimeclient.ObjectKeyFromObject(crd), crd))
			}

			if tt.assertions != nil {
				tt.assertions(t, opts.RuntimeClient)
			}
		})
	}
}

func TestInstallWait(t *testing.T) {
	tests := []struct {
		name string
		objs []runtime.Object
		err  error
	}{
		{
			name: "successful",
			// Seed fake client with already successful deploy
			objs: []runtime.Object{successfulDeploy()},
		},
		{
			name: "failure",
			objs: []runtime.Object{deploy()},
			err:  wait.ErrWaitTimeout,
		},
	}
	for _, tt := range tests {
		testutil.Run(t, tt.name, func(t *testutil.T) {
			// Create fake client and seed with any objs
			client := kfake.NewSimpleClientset(tt.objs...)

			err := deploymentIsReady(context.Background(), "etok", "etok", client, 100*time.Millisecond, 10*time.Millisecond)
			assert.Equal(t, tt.err, err)
		})
	}
}

// Convert []client.Object to []runtime.Object (the CR real client works with
// the former, whereas the CR fake client works with the latter)
func convertObjs(objs ...runtimeclient.Object) (converted []runtime.Object) {
	for _, o := range objs {
		converted = append(converted, o.(runtime.Object))
	}
	return converted
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

func deploy() *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "etok",
			Namespace: "etok",
		},
	}
}

func successfulDeploy() *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "etok",
			Namespace: "etok",
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
