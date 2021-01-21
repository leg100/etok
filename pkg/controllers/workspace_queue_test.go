package controllers

import (
	"testing"

	v1alpha1 "github.com/leg100/etok/api/etok.dev/v1alpha1"
	"github.com/leg100/etok/pkg/testobj"
	"github.com/stretchr/testify/require"
)

func TestUpdateCombinedQueue(t *testing.T) {
	tests := []struct {
		name       string
		workspace  *v1alpha1.Workspace
		runs       []v1alpha1.Run
		wantActive string
		wantQueue  []string
	}{
		{
			name:      "No runs",
			workspace: testobj.Workspace("default", "workspace-1"),
		},
		{
			name:      "One new run",
			workspace: testobj.Workspace("default", "workspace-1"),
			runs: []v1alpha1.Run{
				*testobj.Run("default", "apply-1", "apply", testobj.WithWorkspace("workspace-1")),
			},
			wantActive: "apply-1",
			wantQueue:  []string{},
		},
		{
			name:      "Two new runs",
			workspace: testobj.Workspace("default", "workspace-1"),
			runs: []v1alpha1.Run{
				*testobj.Run("default", "apply-1", "apply", testobj.WithWorkspace("workspace-1")),
				*testobj.Run("default", "apply-2", "apply", testobj.WithWorkspace("workspace-1")),
			},
			wantActive: "apply-1",
			wantQueue:  []string{"apply-2"},
		},
		{
			name:      "One existing run one new run",
			workspace: testobj.Workspace("default", "workspace-1", testobj.WithCombinedQueue("apply-1")),
			runs: []v1alpha1.Run{
				*testobj.Run("default", "apply-1", "apply", testobj.WithWorkspace("workspace-1")),
				*testobj.Run("default", "apply-2", "apply", testobj.WithWorkspace("workspace-1")),
			},
			wantActive: "apply-1",
			wantQueue:  []string{"apply-2"},
		},
		{
			name:      "One completed run",
			workspace: testobj.Workspace("default", "workspace-1", testobj.WithCombinedQueue("apply-1")),
			runs: []v1alpha1.Run{
				*testobj.Run("default", "apply-1", "apply", testobj.WithWorkspace("workspace-1"), testobj.WithCondition(v1alpha1.RunCompleteCondition)),
			},
			wantQueue: []string(nil),
		},
		{
			name:      "One completed run one incomplete run",
			workspace: testobj.Workspace("default", "workspace-1", testobj.WithCombinedQueue("apply-1", "apply-2")),
			runs: []v1alpha1.Run{
				*testobj.Run("default", "apply-1", "apply", testobj.WithWorkspace("workspace-1"), testobj.WithCondition(v1alpha1.RunCompleteCondition)),
				*testobj.Run("default", "apply-2", "apply", testobj.WithWorkspace("workspace-1")),
			},
			wantActive: "apply-2",
			wantQueue:  []string{},
		},
		{
			name:      "Don't queue unqueueable runs",
			workspace: testobj.Workspace("default", "workspace-1"),
			runs: []v1alpha1.Run{
				*testobj.Run("default", "output-1", "output", testobj.WithWorkspace("workspace-1"), testobj.WithRunPhase(v1alpha1.RunPhaseWaiting)),
				*testobj.Run("default", "sh-1", "sh", testobj.WithWorkspace("workspace-1"), testobj.WithRunPhase(v1alpha1.RunPhaseWaiting)),
				*testobj.Run("default", "state-list-1", "list", testobj.WithWorkspace("workspace-1"), testobj.WithRunPhase(v1alpha1.RunPhaseWaiting)),
				*testobj.Run("default", "plan-1", "plan", testobj.WithWorkspace("workspace-1"), testobj.WithRunPhase(v1alpha1.RunPhaseWaiting)),
				*testobj.Run("default", "apply-1", "apply", testobj.WithWorkspace("workspace-1"), testobj.WithRunPhase(v1alpha1.RunPhaseWaiting)),
			},
			wantActive: "sh-1",
			wantQueue:  []string{"apply-1"},
		},
		{
			name:      "Unapproved privileged command",
			workspace: testobj.Workspace("", "workspace-1", testobj.WithPrivilegedCommands("apply")),
			runs: []v1alpha1.Run{
				*testobj.Run("default", "apply-1", "apply", testobj.WithWorkspace("workspace-1")),
			},
			wantQueue: []string(nil),
		},
		{
			name:      "Approved privileged command",
			workspace: testobj.Workspace("", "workspace-1", testobj.WithPrivilegedCommands("apply"), testobj.WithApprovals("apply-1")),
			runs: []v1alpha1.Run{
				*testobj.Run("default", "apply-1", "apply", testobj.WithWorkspace("workspace-1")),
			},
			wantActive: "apply-1",
			wantQueue:  []string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			updateCombinedQueue(tt.workspace, tt.runs)
			require.Equal(t, tt.wantActive, tt.workspace.Status.Active)
			require.Equal(t, tt.wantQueue, tt.workspace.Status.Queue)
		})
	}
}
