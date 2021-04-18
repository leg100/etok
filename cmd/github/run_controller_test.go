package github

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/leg100/etok/api/etok.dev/v1alpha1"
	"github.com/leg100/etok/pkg/scheme"
	"github.com/leg100/etok/pkg/testobj"
	"github.com/leg100/etok/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestRunController(t *testing.T) {
	tests := []struct {
		name       string
		run        *v1alpha1.Run
		objs       []runtime.Object
		assertions func(*testutil.T, *v1alpha1.Run, *checkRun)
	}{
		{
			name: "Defaults",
			run: testobj.Run("default", "default", "plan",
				testobj.WithWorkspace("workspace-1"),
				testobj.WithLabels(
					githubAppInstallIDLabelName, "123",
					checkRunIDLabelName, "456",
					checkRunSHALabelName, "da39a3ee5e6b4b0d3255bfef95601890afd80709",
					checkRunOwnerLabelName, "leg100",
					checkRunRepoLabelName, "etok",
					checkRunCommandLabelName, "plan",
					checkRunIterationLabelName, "1",
				),
			),
			assertions: func(t *testutil.T, run *v1alpha1.Run, cr *checkRun) {
				assert.NotNil(t, cr)
			},
		},
		{
			name: "Run pod succeeded",
			run: testobj.Run("default", "default", "plan",
				testobj.WithWorkspace("workspace-1"),
				testobj.WithLabels(
					githubAppInstallIDLabelName, "123",
					checkRunIDLabelName, "456",
					checkRunSHALabelName, "da39a3ee5e6b4b0d3255bfef95601890afd80709",
					checkRunOwnerLabelName, "leg100",
					checkRunRepoLabelName, "etok",
					checkRunCommandLabelName, "plan",
					checkRunIterationLabelName, "1",
				),
				testobj.WithCondition(v1alpha1.RunCompleteCondition, v1alpha1.PodSucceededReason),
			),
			assertions: func(t *testutil.T, run *v1alpha1.Run, cr *checkRun) {
				assert.Contains(t, "completed", run.GetLabels()[checkRunStatusLabelName])
				assert.Equal(t, "fake logs", string(cr.out))
			},
		},
		{
			name: "Skip completed run",
			run: testobj.Run("default", "default", "plan",
				testobj.WithWorkspace("workspace-1"),
				testobj.WithLabels(
					checkRunStatusLabelName, "completed",
				),
			),
			assertions: func(t *testutil.T, run *v1alpha1.Run, cr *checkRun) {
				// No check run should have been constructed
				assert.Nil(t, cr)
			},
		},
	}
	for _, tt := range tests {
		testutil.Run(t, tt.name, func(t *testutil.T) {
			objs := append(tt.objs, runtime.Object(tt.run))
			client := fake.NewClientBuilder().WithRuntimeObjects(objs...).WithScheme(scheme.Scheme).Build()

			sender := &fakeSender{}
			reconciler := &runReconciler{
				Client:   client,
				sender:   sender,
				streamer: &fakeStreamer{},
			}

			req := requestFromObject(tt.run)
			_, err := reconciler.Reconcile(context.Background(), req)
			require.NoError(t, err)

			if tt.assertions != nil {
				// Fetch latest run because the reconciler may have updated it
				var run v1alpha1.Run
				require.NoError(t, client.Get(context.Background(), req.NamespacedName, &run))

				tt.assertions(t, &run, sender.cr)
			}
		})
	}
}

type fakeSender struct {
	cr *checkRun
}

func (s *fakeSender) send(_ int64, inv invoker) error {
	s.cr = inv.(*checkRun)

	return nil
}

type fakeStreamer struct{}

func (s *fakeStreamer) Stream(ctx context.Context, key client.ObjectKey) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewBufferString("fake logs")), nil
}
