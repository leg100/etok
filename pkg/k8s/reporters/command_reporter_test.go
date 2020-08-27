package reporters

import (
	"testing"
	"time"

	"github.com/leg100/stok/api/command"
	"github.com/leg100/stok/api/v1alpha1"
	"github.com/leg100/stok/controllers"
	"github.com/leg100/stok/scheme"
	"github.com/stretchr/testify/require"
	"gotest.tools/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestCommandReporter(t *testing.T) {
	tests := []struct {
		name       string
		cmd        command.Interface
		assertions func(exit bool, enqueueTimer, queueTimer *time.Timer)
	}{
		{
			name: "pending",
			cmd: &v1alpha1.Plan{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "plan-1",
					Namespace: "operator-test",
				},
				CommandSpec: v1alpha1.CommandSpec{
					Workspace: "workspace-1",
				},
				CommandStatus: v1alpha1.CommandStatus{
					Phase: v1alpha1.CommandPhasePending,
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
			cmd: &v1alpha1.Plan{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "plan-1",
					Namespace: "operator-test",
				},
				CommandSpec: v1alpha1.CommandSpec{
					Workspace: "workspace-1",
				},
				CommandStatus: v1alpha1.CommandStatus{
					Phase: v1alpha1.CommandPhaseQueued,
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
			cmd: &v1alpha1.Plan{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "plan-1",
					Namespace: "operator-test",
				},
				CommandSpec: v1alpha1.CommandSpec{
					Workspace: "workspace-1",
				},
				CommandStatus: v1alpha1.CommandStatus{
					Phase: v1alpha1.CommandPhaseSync,
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
		c := fake.NewFakeClientWithScheme(s, tt.cmd)
		kind, _ := controllers.GetKindFromObject(s, tt.cmd)
		reporter := &CommandReporter{
			Id:     "plan-1",
			Client: c,
			Kind:   kind,
		}

		req := ctrl.Request{
			NamespacedName: types.NamespacedName{
				Name:      tt.cmd.GetName(),
				Namespace: tt.cmd.GetNamespace(),
			},
		}

		enqueueTimer := time.NewTimer(time.Second)
		queueTimer := time.NewTimer(time.Second)

		exit, err := reporter.report(req, enqueueTimer, queueTimer)
		require.NoError(t, err)

		tt.assertions(exit, enqueueTimer, queueTimer)
	}
}
