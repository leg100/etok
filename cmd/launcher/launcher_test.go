package launcher

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"testing"

	"github.com/leg100/stok/api/run"
	"github.com/leg100/stok/api/stok.goalspike.com/v1alpha1"
	"github.com/leg100/stok/pkg/env"
	"github.com/leg100/stok/pkg/k8s"
	"github.com/leg100/stok/pkg/k8s/stokclient"
	"github.com/leg100/stok/pkg/k8s/stokclient/fake"
	"github.com/leg100/stok/pkg/launcher"
	"github.com/leg100/stok/testutil"
	"github.com/operator-framework/operator-sdk/pkg/status"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	kfake "k8s.io/client-go/kubernetes/fake"
)

func TestParseTerraformArgs(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		stokargs []string
		tfargs   []string
	}{
		{
			name:     "no args",
			args:     []string{},
			stokargs: []string{},
			tfargs:   []string{},
		},
		{
			name:     "tf args, no stok args",
			args:     []string{"plan", "-input", "false"},
			stokargs: []string{"plan"},
			tfargs:   []string{"-input", "false"},
		},
		{
			name:     "stok args, no tf args",
			args:     []string{"plan", "--", "--namespace", "foo"},
			stokargs: []string{"plan", "--namespace", "foo"},
			tfargs:   []string{},
		},
		{
			name:     "stok args, and tf args",
			args:     []string{"plan", "-input", "false", "--", "--namespace", "foo"},
			stokargs: []string{"plan", "--namespace", "foo"},
			tfargs:   []string{"-input", "false"},
		},
		{
			name:     "not a launcher command",
			args:     []string{"workspace", "new", "bar", "--namespace", "foo"},
			stokargs: []string{"workspace", "new", "bar", "--namespace", "foo"},
			tfargs:   []string{},
		},
	}

	for _, tt := range tests {
		testutil.Run(t, tt.name, func(t *testutil.T) {
			stokargs, tfargs := parseTerraformArgs(tt.args)

			require.Equal(t, tt.stokargs, stokargs)
			require.Equal(t, tt.tfargs, tfargs)
		})
	}
}

func TestTerraform(t *testing.T) {
	workspaceObj := func(namespace, name string, queue ...string) *v1alpha1.Workspace {
		return &v1alpha1.Workspace{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			Status: v1alpha1.WorkspaceStatus{
				Conditions: status.Conditions{
					{
						Type:   v1alpha1.ConditionHealthy,
						Status: corev1.ConditionTrue,
					},
				},
				Queue: queue,
			},
		}
	}

	podReadyAndRunning := func(namespace, name string) *corev1.Pod {
		return &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			Status: corev1.PodStatus{
				Phase: corev1.PodRunning,
				Conditions: []corev1.PodCondition{
					{
						Type:   corev1.PodReady,
						Status: corev1.ConditionTrue,
					},
				},
			},
		}
	}

	tests := []struct {
		name     string
		args     []string
		env      env.StokEnv
		stokObjs []runtime.Object
		kubeObjs []runtime.Object
		err      string
		code     int
	}{}

	for _, tfcmd := range run.TerraformCommands {
		tests = append(tests, []struct {
			name     string
			args     []string
			env      env.StokEnv
			stokObjs []runtime.Object
			kubeObjs []runtime.Object
			err      string
			code     int
		}{
			{
				name:     tfcmd + "WithDefaults",
				args:     []string{tfcmd, "--", "--debug"},
				env:      env.StokEnv("default/default"),
				kubeObjs: []runtime.Object{podReadyAndRunning("default", "run-12345")},
				stokObjs: []runtime.Object{workspaceObj("default", "default")},
			},
			{
				name:     tfcmd + "WithSpecificNamespaceAndWorkspace",
				args:     []string{tfcmd, "--", "--debug"},
				env:      env.StokEnv("foo/bar"),
				kubeObjs: []runtime.Object{podReadyAndRunning("foo", "run-12345")},
				stokObjs: []runtime.Object{workspaceObj("foo", "bar")},
			},
			{
				name:     tfcmd + "WithSpecificNamespaceAndWorkspaceFlags",
				args:     []string{tfcmd, "--", "--debug", "--namespace", "foo", "--workspace", "bar"},
				env:      env.StokEnv("default/default"),
				kubeObjs: []runtime.Object{podReadyAndRunning("foo", "run-12345")},
				stokObjs: []runtime.Object{workspaceObj("foo", "bar")},
			},
			{
				name:     tfcmd + "WithTerraformFlag",
				args:     []string{tfcmd, "-input", "false", "--", "--debug"},
				env:      env.StokEnv("default/default"),
				kubeObjs: []runtime.Object{podReadyAndRunning("default", "run-12345")},
				stokObjs: []runtime.Object{workspaceObj("default", "default")},
			},
			{
				name:     tfcmd + "WithContextFlag",
				args:     []string{tfcmd, "--", "--debug", "--context", "oz-cluster"},
				env:      env.StokEnv("default/default"),
				kubeObjs: []runtime.Object{podReadyAndRunning("default", "run-12345")},
				stokObjs: []runtime.Object{workspaceObj("default", "default")},
			},
			{
				name:     tfcmd + "WithoutStokEnv",
				args:     []string{tfcmd},
				kubeObjs: []runtime.Object{podReadyAndRunning("default", "run-12345")},
				stokObjs: []runtime.Object{workspaceObj("default", "default")},
			},
		}...)
	}

	for _, tt := range tests {
		testutil.Run(t, tt.name, func(t *testutil.T) {
			// Change into temp dir just for this test
			path := t.NewTempDir().Chdir().Root()

			// Write .terraform/environment
			if tt.env != "" {
				require.NoError(t, tt.env.Write(path))
			}

			// Fix name for run, configmap, and pod
			name := "run-12345"
			t.Override(&launcher.GenerateName, func() string { return name })

			// Mock call to attach to pod TTY
			t.Override(&k8s.Attach, func(pod *corev1.Pod) error { return nil })

			// Mock call to retrieve pod logs
			t.Override(&k8s.GetLogs, func(ctx context.Context, kc kubernetes.Interface, pod *corev1.Pod, container string) (io.ReadCloser, error) {
				return ioutil.NopCloser(bytes.NewBufferString("test output")), nil
			})

			// Populate fake kubernetes client with relevant objects
			fakeKubeClient := kfake.NewSimpleClientset(tt.kubeObjs...)
			t.Override(&k8s.KubeClient, func() (kubernetes.Interface, error) {
				return fakeKubeClient, nil
			})

			// Mock run controller: update phase on run obj
			// watcher := watch.NewFake()
			// fakeKubeClient.PrependWatchReactor("runs", testcore.DefaultWatchReactor(watcher, nil))
			// watcher.Modify(run("default"))

			// Populate fake stok client with relevant objects
			fakeStokClient := fake.NewSimpleClientset(tt.stokObjs...)
			t.Override(&k8s.StokClient, func() (stokclient.Interface, error) {
				return fakeStokClient, nil
			})

			// Execute cobra command
			out := new(bytes.Buffer)
			code, err := newStokCmd(tt.args, out, out).Execute()

			if tt.err != "" {
				require.EqualError(t, err, tt.err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tt.code, code)
		})

	}
}
