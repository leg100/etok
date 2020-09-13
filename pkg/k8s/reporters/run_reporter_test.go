package reporters

import (
	"testing"
	"time"

	"github.com/leg100/stok/api/v1alpha1"
	"github.com/leg100/stok/scheme"
	"github.com/stretchr/testify/require"
	"gotest.tools/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestRunReporter(t *testing.T) {
	tests := []struct {
		name       string
		run        *v1alpha1.Run
		assertions func(exit bool, enqueueTimer, queueTimer *time.Timer)
	}{
		{
			name: "pending",
			run: &v1alpha1.Run{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "plan-1",
					Namespace: "operator-test",
				},
				RunSpec: v1alpha1.RunSpec{
					Workspace: "workspace-1",
				},
				RunStatus: v1alpha1.RunStatus{
					Phase: v1alpha1.RunPhasePending,
				},
			},
			assertions: func(exit bool, enqueueTimer, queueTimer *time.Timer) {
				assert.Equal(t, false, exit)
				// Assert enqueue timer has not been stopped yet
				assert.Equal(t, true, enqueueTimer.Stop())
				// Assert queue timer has not been stopped yet
				assert.Equal(t, true, queueTimer.Stop())
			},
		},
		{
			name: "queued",
			run: &v1alpha1.Run{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "plan-1",
					Namespace: "operator-test",
				},
				RunSpec: v1alpha1.RunSpec{
					Workspace: "workspace-1",
				},
				RunStatus: v1alpha1.RunStatus{
					Phase: v1alpha1.RunPhaseQueued,
				},
			},
			assertions: func(exit bool, enqueueTimer, queueTimer *time.Timer) {
				assert.Equal(t, false, exit)
				// Assert enqueue timer has already been stopped
				assert.Equal(t, false, enqueueTimer.Stop())
			},
		},
		{
			name: "synchronising",
			run: &v1alpha1.Run{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "plan-1",
					Namespace: "operator-test",
				},
				RunSpec: v1alpha1.RunSpec{
					Workspace: "workspace-1",
				},
				RunStatus: v1alpha1.RunStatus{
					Phase: v1alpha1.RunPhaseSync,
				},
			},
			assertions: func(exit bool, enqueueTimer, queueTimer *time.Timer) {
				assert.Equal(t, true, exit)
				// Assert enqueue timer has already been stopped
				assert.Equal(t, false, enqueueTimer.Stop())
				// Assert queue timer has already been stopped
				assert.Equal(t, false, queueTimer.Stop())
			},
		},
	}

	for _, tt := range tests {
		s := scheme.Scheme
		c := fake.NewFakeClientWithScheme(s, tt.run)
		reporter := &RunReporter{
			Id:     "plan-1",
			Client: c,
		}

		req := ctrl.Request{
			NamespacedName: types.NamespacedName{
				Name:      tt.run.GetName(),
				Namespace: tt.run.GetNamespace(),
			},
		}

		enqueueTimer := time.NewTimer(time.Second)
		queueTimer := time.NewTimer(time.Second)

		exit, err := reporter.report(req, enqueueTimer, queueTimer)
		require.NoError(t, err)

		tt.assertions(exit, enqueueTimer, queueTimer)
	}
}
