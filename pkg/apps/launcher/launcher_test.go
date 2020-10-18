package launcher

import (
	"bytes"
	"context"
	"testing"

	"github.com/leg100/stok/api/stok.goalspike.com/v1alpha1"
	"github.com/leg100/stok/controllers"
	"github.com/leg100/stok/pkg/app"
	"github.com/leg100/stok/pkg/k8s/stokclient/fake"
	"github.com/leg100/stok/pkg/podhandler"
	"github.com/leg100/stok/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	testcore "k8s.io/client-go/testing"
)

func TestLauncher(t *testing.T) {
	tests := []struct {
		name       string
		err        bool
		setOpts    func(opts *app.Options)
		assertions func(opts *app.Options)
	}{
		{
			name: "defaults",
		},
		{
			name: "non-default namespace",
			setOpts: func(opts *app.Options) {
				opts.Namespace = "dev"
			},
		},
		{
			name: "plan with args",
			setOpts: func(opts *app.Options) {
				opts.Command = "Plan"
				opts.Args = []string{"-input", "false"}
			},
		},
	}

	for _, tt := range tests {
		testutil.Run(t, tt.name, func(t *testutil.T) {
			opts, err := app.NewFakeOptsWithClients(new(bytes.Buffer))
			require.NoError(t, err)

			if tt.setOpts != nil {
				tt.setOpts(opts)
			}

			mockRunController(opts)

			err = (&Launcher{Options: opts, PodHandler: &podhandler.PodHandlerFake{}}).Run(context.Background())
			assert.NoError(t, err)

			// Confirm run exists
			run, err := opts.StokClient().StokV1alpha1().Runs(opts.Namespace).Get(context.Background(), opts.Name, metav1.GetOptions{})
			assert.NoError(t, err)

			// Confirm wait annotation key has been deleted
			assert.False(t, controllers.IsSynchronising(run))

			assert.Equal(t, opts.Namespace, run.GetNamespace())
			assert.Equal(t, opts.Name, run.GetName())
			assert.Equal(t, opts.Command, run.GetCommand())
			assert.Equal(t, opts.Args, run.GetArgs())

			if tt.assertions != nil {
				tt.assertions(opts)
			}
		})
	}
}

// When a run create event occurs create a pod
func mockRunController(opts *app.Options) {
	createPodAction := func(action testcore.Action) (bool, runtime.Object, error) {
		run := action.(testcore.CreateAction).GetObject().(*v1alpha1.Run)
		pod := testPod(run.GetNamespace(), run.GetName())
		opts.KubeClient().CoreV1().Pods(run.GetNamespace()).Create(context.Background(), pod, metav1.CreateOptions{})

		return false, action.(testcore.CreateAction).GetObject(), nil
	}

	opts.StokClient().(*fake.Clientset).PrependReactor("create", "runs", createPodAction)
}

func testPod(namespace, name string) *corev1.Pod {
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
