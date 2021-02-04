package install

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	appsv1 "k8s.io/api/apps/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"

	cmdutil "github.com/leg100/etok/cmd/util"
	"github.com/leg100/etok/pkg/backup"
	etokclient "github.com/leg100/etok/pkg/client"
	"github.com/leg100/etok/pkg/scheme"
	"github.com/leg100/etok/pkg/testobj"
	"github.com/leg100/etok/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestInstall(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		objs       []runtimeclient.Object
		err        error
		assertions func(*testutil.T, runtimeclient.Client)
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
			name: "upgrade",
			args: []string{"install", "--wait=false"},
			objs: append(wantedResources(), wantedCRDs()...),
		},
		{
			name: "fresh local install",
			args: []string{"install", "--local", "--wait=false"},
		},
		{
			name: "fresh install with service account annotations",
			args: []string{"install", "--wait=false", "--sa-annotations", "foo=bar,baz=haj"},
			assertions: func(t *testutil.T, client runtimeclient.Client) {
				var sa corev1.ServiceAccount
				client.Get(context.Background(), types.NamespacedName{Namespace: "etok", Name: "etok"}, &sa)
				assert.Equal(t, map[string]string{"foo": "bar", "baz": "haj"}, sa.GetAnnotations())
			},
		},
		{
			name: "fresh install with custom image",
			args: []string{"install", "--wait=false", "--image", "bugsbunny:v123"},
			assertions: func(t *testutil.T, client runtimeclient.Client) {
				var d appsv1.Deployment
				client.Get(context.Background(), types.NamespacedName{Namespace: defaultNamespace, Name: "etok"}, &d)

				assert.Equal(t, "bugsbunny:v123", d.Spec.Template.Spec.Containers[0].Image)
			},
		},
		{
			name: "fresh install with secret found",
			args: []string{"install", "--wait=false"},
			objs: []runtimeclient.Object{testobj.Secret("etok", "etok")},
			assertions: func(t *testutil.T, client runtimeclient.Client) {
				var d appsv1.Deployment
				client.Get(context.Background(), types.NamespacedName{Namespace: defaultNamespace, Name: "etok"}, &d)

				assert.Contains(t, d.Spec.Template.Spec.Containers[0].EnvFrom, corev1.EnvFromSource{
					SecretRef: &corev1.SecretEnvSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "etok",
						},
					},
				})
			},
		},
		{
			name: "fresh install with backups enabled",
			args: []string{"install", "--wait=false", "--backup-provider=gcs", "--gcs-bucket=backups-bucket"},
			assertions: func(t *testutil.T, client runtimeclient.Client) {
				var d appsv1.Deployment
				client.Get(context.Background(), types.NamespacedName{Namespace: defaultNamespace, Name: "etok"}, &d)

				assert.Contains(t, d.Spec.Template.Spec.Containers[0].Env, corev1.EnvVar{Name: "ETOK_BACKUP_PROVIDER", Value: "gcs"})
				assert.Contains(t, d.Spec.Template.Spec.Containers[0].Env, corev1.EnvVar{Name: "ETOK_GCS_BUCKET", Value: "backups-bucket"})
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
	}
	for _, tt := range tests {
		testutil.Run(t, tt.name, func(t *testutil.T) {
			// When retrieve local paths to YAML files, it's assumed the user's
			// pwd is the repo root
			t.Chdir("../../")

			buf := new(bytes.Buffer)
			f := &cmdutil.Factory{
				IOStreams:            cmdutil.IOStreams{Out: os.Stdout},
				RuntimeClientCreator: NewFakeClientCreator(convertObjs(tt.objs...)...),
			}

			cmd, opts := InstallCmd(f)
			cmd.SetOut(buf)
			cmd.SetArgs(tt.args)

			// Mock a remote web server from which YAML files will be retrieved
			mockWebServer(t)

			// Override wait interval to ensure fast tests
			t.Override(&interval, 10*time.Millisecond)

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

			// get runtime client now that it's been created
			client := opts.RuntimeClient

			// assert CRDs are present
			for _, res := range wantedCRDs() {
				assert.NoError(t, client.Get(context.Background(), runtimeclient.ObjectKeyFromObject(res), res))
			}

			// assert non-CRD resources are present
			if !opts.crdsOnly {
				for _, res := range wantedResources() {
					assert.NoError(t, client.Get(context.Background(), runtimeclient.ObjectKeyFromObject(res), res))
				}
			}

			if tt.assertions != nil {
				tt.assertions(t, client)
			}
		})
	}
}

func TestInstallWait(t *testing.T) {
	tests := []struct {
		name string
		objs []runtimeclient.Object
		err  error
	}{
		{
			name: "successful",
			// Seed fake client with already successful deploy
			objs: []runtimeclient.Object{successfulDeploy()},
		},
		{
			name: "failure",
			objs: []runtimeclient.Object{deploy()},
			err:  wait.ErrWaitTimeout,
		},
	}
	for _, tt := range tests {
		testutil.Run(t, tt.name, func(t *testutil.T) {
			// Override wait interval to ensure fast tests
			t.Override(&interval, 10*time.Millisecond)

			// Create fake client and seed with any objs
			client := fake.NewFakeClientWithScheme(scheme.Scheme, convertObjs(tt.objs...)...)
			opts := &installOptions{
				Client: &etokclient.Client{
					RuntimeClient: client,
				},
				Factory: &cmdutil.Factory{
					IOStreams: cmdutil.IOStreams{Out: new(bytes.Buffer)},
				},
				timeout: 100 * time.Millisecond,
			}
			assert.Equal(t, tt.err, opts.deploymentIsReady(context.Background(), deploy()))
		})
	}
}

func TestInstallDryRun(t *testing.T) {
	testutil.Run(t, "default", func(t *testutil.T) {
		// When retrieve local paths to YAML files, it's assumed the user's pwd
		// is the repo root
		t.Chdir("../../")

		out := new(bytes.Buffer)
		opts := &installOptions{
			backupCfg: backup.NewConfig(),
			Client: &etokclient.Client{
				RuntimeClient: fake.NewFakeClientWithScheme(scheme.Scheme),
			},
			Factory: &cmdutil.Factory{
				IOStreams: cmdutil.IOStreams{Out: out},
			},
			dryRun: true,
			local:  true,
		}
		require.NoError(t, opts.install(context.Background()))

		docs := strings.Split(out.String(), "---\n")
		assert.Equal(t, 11, len(docs))
	})
}

// Convert []client.Object to []runtime.Object (the CR real client works with
// the former, whereas the CR fake client works with the latter)
func convertObjs(objs ...runtimeclient.Object) (converted []runtime.Object) {
	for _, o := range objs {
		converted = append(converted, o.(runtime.Object))
	}
	return
}

func wantedCRDs() (resources []runtimeclient.Object) {
	resources = append(resources, &apiextv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: "workspaces.etok.dev"}})
	resources = append(resources, &apiextv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: "runs.etok.dev"}})
	return
}

func wantedResources() (resources []runtimeclient.Object) {
	resources = append(resources, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "etok"}})
	resources = append(resources, &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Namespace: "etok", Name: "etok"}})
	resources = append(resources, &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: "etok"}})
	resources = append(resources, &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: "etok-user"}})
	resources = append(resources, &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: "etok-admin"}})
	resources = append(resources, &rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "etok"}})
	resources = append(resources, &rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "etok-user"}})
	resources = append(resources, &rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "etok-admin"}})
	resources = append(resources, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: "etok", Name: "etok"}})
	return
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

func mockWebServer(t *testutil.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Respond by reading the request path from local FS (the path is made
		// relative by stripping off the first '/')
		respondWithFile(w, r.URL.Path[1:])
	}))
	t.Override(&repoURL, ts.URL)
	t.Cleanup(ts.Close)
}

func respondWithFile(w io.Writer, path string) {
	body, _ := ioutil.ReadFile(path)
	fmt.Fprintln(w, string(body))
}
