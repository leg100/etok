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
	plan1 := v1alpha1.Run{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "plan-1",
			Namespace: "default",
		},
		RunSpec: v1alpha1.RunSpec{
			Command:   "plan",
			Workspace: "workspace-1",
		},
	}

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
			name:      "Single run",
			workspace: testWorkspaceQueue("default", "workspace-1"),
			runs:      []runtime.Object{&plan1},
			assertions: func(queue []string) {
				require.Equal(t, []string{"plan-1"}, queue)
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

func WithPrivilegedCommands(cmds []string) func(*v1alpha1.Workspace) {
	return func(ws *v1alpha1.Workspace) {
		ws.Spec.PrivilegedCommands = cmds
	}
}
