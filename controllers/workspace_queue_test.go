package controllers

import (
	"context"
	"testing"

	v1alpha1 "github.com/leg100/stok/api/stok.goalspike.com/v1alpha1"
	"github.com/leg100/stok/scheme"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestUpdateQueue(t *testing.T) {
	tests := []struct {
		name       string
		workspace  *v1alpha1.Workspace
		runs       []runtime.Object
		assertions func([]string)
	}{
		{
			name:      "No runs",
			workspace: testWorkspaceQueue("default", "workspace-1"),
			runs:      []runtime.Object{},
			assertions: func(queue []string) {
				require.Equal(t, []string{}, queue)
			},
		},
		{
			name:      "One new run",
			workspace: testWorkspaceQueue("default", "workspace-1"),
			runs: []runtime.Object{
				testRun("default", "plan-1", "plan", "workspace-1"),
			},
			assertions: func(queue []string) {
				require.Equal(t, []string{"plan-1"}, queue)
			},
		},
		{
			name:      "Two new runs",
			workspace: testWorkspaceQueue("default", "workspace-1"),
			runs: []runtime.Object{
				testRun("default", "plan-1", "plan", "workspace-1"),
				testRun("default", "plan-2", "plan", "workspace-1"),
			},
			assertions: func(queue []string) {
				require.Equal(t, []string{"plan-1", "plan-2"}, queue)
			},
		},
		{
			name:      "One existing run one new run",
			workspace: testWorkspaceQueue("default", "workspace-1", WithExistingQueue([]string{"plan-1"})),
			runs: []runtime.Object{
				testRun("default", "plan-1", "plan", "workspace-1"),
				testRun("default", "plan-2", "plan", "workspace-1"),
			},
			assertions: func(queue []string) {
				require.Equal(t, []string{"plan-1", "plan-2"}, queue)
			},
		},
		{
			name:      "One completed run",
			workspace: testWorkspaceQueue("default", "workspace-1", WithExistingQueue([]string{"plan-1"})),
			runs: []runtime.Object{
				testRun("default", "plan-1", "plan", "workspace-1", WithPhase(v1alpha1.RunPhaseCompleted)),
			},
			assertions: func(queue []string) {
				require.Equal(t, []string{}, queue)
			},
		},
		{
			name:      "One completed run one running run",
			workspace: testWorkspaceQueue("default", "workspace-1", WithExistingQueue([]string{"plan-1", "plan-2"})),
			runs: []runtime.Object{
				testRun("default", "plan-1", "plan", "workspace-1", WithPhase(v1alpha1.RunPhaseCompleted)),
				testRun("default", "plan-2", "plan", "workspace-1", WithPhase(v1alpha1.RunPhaseRunning)),
			},
			assertions: func(queue []string) {
				require.Equal(t, []string{"plan-2"}, queue)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objs := append(tt.runs, runtime.Object(tt.workspace))
			c := fake.NewFakeClientWithScheme(scheme.Scheme, objs...)

			require.NoError(t, updateQueue(c, tt.workspace))

			// Fetch fresh workspace for assertions
			ws := &v1alpha1.Workspace{}
			key := types.NamespacedName{Namespace: tt.workspace.Namespace, Name: tt.workspace.Name}
			require.NoError(t, c.Get(context.TODO(), key, ws))

			tt.assertions(ws.Status.Queue)
		})
	}
}

func testWorkspaceQueue(namespace, name string, opts ...func(*v1alpha1.Workspace)) *v1alpha1.Workspace {
	ws := &v1alpha1.Workspace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	for _, o := range opts {
		o(ws)
	}
	return ws
}

func WithExistingQueue(runs []string) func(*v1alpha1.Workspace) {
	return func(ws *v1alpha1.Workspace) {
		ws.Status.Queue = runs
	}
}

func testRun(namespace, name, command, workspace string, opts ...func(*v1alpha1.Run)) *v1alpha1.Run {
	run := &v1alpha1.Run{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		RunSpec: v1alpha1.RunSpec{
			Command:   command,
			Workspace: workspace,
		},
	}
	for _, o := range opts {
		o(run)
	}
	return run
}

func WithPhase(phase v1alpha1.RunPhase) func(*v1alpha1.Run) {
	return func(run *v1alpha1.Run) {
		run.Phase = phase
	}
}
