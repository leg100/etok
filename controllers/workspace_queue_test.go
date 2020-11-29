package controllers

import (
	"context"
	"testing"

	v1alpha1 "github.com/leg100/stok/api/stok.goalspike.com/v1alpha1"
	"github.com/leg100/stok/pkg/testobj"
	"github.com/leg100/stok/scheme"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
			workspace: testobj.Workspace("default", "workspace-1"),
			runs:      []runtime.Object{},
			assertions: func(queue []string) {
				assert.Equal(t, []string{}, queue)
			},
		},
		{
			name:      "One new run",
			workspace: testobj.Workspace("default", "workspace-1"),
			runs: []runtime.Object{
				testobj.Run("default", "plan-1", "plan", testobj.WithWorkspace("workspace-1")),
			},
			assertions: func(queue []string) {
				require.Equal(t, []string{"plan-1"}, queue)
			},
		},
		{
			name:      "Two new runs",
			workspace: testobj.Workspace("default", "workspace-1"),
			runs: []runtime.Object{
				testobj.Run("default", "plan-1", "plan", testobj.WithWorkspace("workspace-1")),
				testobj.Run("default", "plan-2", "plan", testobj.WithWorkspace("workspace-1")),
			},
			assertions: func(queue []string) {
				assert.Equal(t, []string{"plan-1", "plan-2"}, queue)
			},
		},
		{
			name:      "One existing run one new run",
			workspace: testobj.Workspace("default", "workspace-1", testobj.WithQueue("plan-1")),
			runs: []runtime.Object{
				testobj.Run("default", "plan-1", "plan", testobj.WithWorkspace("workspace-1")),
				testobj.Run("default", "plan-2", "plan", testobj.WithWorkspace("workspace-1")),
			},
			assertions: func(queue []string) {
				assert.Equal(t, []string{"plan-1", "plan-2"}, queue)
			},
		},
		{
			name:      "One completed run",
			workspace: testobj.Workspace("default", "workspace-1", testobj.WithQueue("plan-1")),
			runs: []runtime.Object{
				testobj.Run("default", "plan-1", "plan", testobj.WithWorkspace("workspace-1"), testobj.WithRunPhase(v1alpha1.RunPhaseCompleted)),
			},
			assertions: func(queue []string) {
				assert.Equal(t, []string{}, queue)
			},
		},
		{
			name:      "One completed run one running run",
			workspace: testobj.Workspace("default", "workspace-1", testobj.WithQueue("plan-1", "plan-2")),
			runs: []runtime.Object{
				testobj.Run("default", "plan-1", "plan", testobj.WithWorkspace("workspace-1"), testobj.WithRunPhase(v1alpha1.RunPhaseCompleted)),
				testobj.Run("default", "plan-2", "plan", testobj.WithWorkspace("workspace-1"), testobj.WithRunPhase(v1alpha1.RunPhaseRunning)),
			},
			assertions: func(queue []string) {
				assert.Equal(t, []string{"plan-2"}, queue)
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
