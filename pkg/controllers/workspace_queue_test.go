package controllers

import (
	"testing"

	v1alpha1 "github.com/leg100/etok/api/etok.dev/v1alpha1"
	"github.com/leg100/etok/pkg/testobj"
	"github.com/stretchr/testify/require"
)

func TestUpdateQueue(t *testing.T) {
	tests := []struct {
		name      string
		workspace *v1alpha1.Workspace
		runs      []v1alpha1.Run
		want      []string
	}{
		{
			name:      "No runs",
			workspace: testobj.Workspace("default", "workspace-1"),
			want:      []string(nil),
		},
		{
			name:      "One new run",
			workspace: testobj.Workspace("default", "workspace-1"),
			runs: []v1alpha1.Run{
				*testobj.Run("default", "apply-1", "apply", testobj.WithWorkspace("workspace-1")),
			},
			want: []string{"apply-1"},
		},
		{
			name:      "Two new runs",
			workspace: testobj.Workspace("default", "workspace-1"),
			runs: []v1alpha1.Run{
				*testobj.Run("default", "apply-1", "apply", testobj.WithWorkspace("workspace-1")),
				*testobj.Run("default", "apply-2", "apply", testobj.WithWorkspace("workspace-1")),
			},
			want: []string{"apply-1", "apply-2"},
		},
		{
			name:      "One existing run one new run",
			workspace: testobj.Workspace("default", "workspace-1", testobj.WithQueue("apply-1")),
			runs: []v1alpha1.Run{
				*testobj.Run("default", "apply-1", "apply", testobj.WithWorkspace("workspace-1")),
				*testobj.Run("default", "apply-2", "apply", testobj.WithWorkspace("workspace-1")),
			},
			want: []string{"apply-1", "apply-2"},
		},
		{
			name:      "One completed run",
			workspace: testobj.Workspace("default", "workspace-1", testobj.WithQueue("apply-1")),
			runs: []v1alpha1.Run{
				*testobj.Run("default", "apply-1", "apply", testobj.WithWorkspace("workspace-1"), testobj.WithRunPhase(v1alpha1.RunPhaseCompleted)),
			},
			want: []string(nil),
		},
		{
			name:      "One completed run one running run",
			workspace: testobj.Workspace("default", "workspace-1", testobj.WithQueue("apply-1", "apply-2")),
			runs: []v1alpha1.Run{
				*testobj.Run("default", "apply-1", "apply", testobj.WithWorkspace("workspace-1"), testobj.WithRunPhase(v1alpha1.RunPhaseCompleted)),
				*testobj.Run("default", "apply-2", "apply", testobj.WithWorkspace("workspace-1"), testobj.WithRunPhase(v1alpha1.RunPhaseRunning)),
			},
			want: []string{"apply-2"},
		},
		{
			name:      "Don't queue unqueueable runs",
			workspace: testobj.Workspace("default", "workspace-1"),
			runs: []v1alpha1.Run{
				*testobj.Run("default", "output-1", "output", testobj.WithWorkspace("workspace-1"), testobj.WithRunPhase(v1alpha1.RunPhasePending)),
				*testobj.Run("default", "sh-1", "sh", testobj.WithWorkspace("workspace-1"), testobj.WithRunPhase(v1alpha1.RunPhasePending)),
				*testobj.Run("default", "state-list-1", "list", testobj.WithWorkspace("workspace-1"), testobj.WithRunPhase(v1alpha1.RunPhasePending)),
				*testobj.Run("default", "plan-1", "plan", testobj.WithWorkspace("workspace-1"), testobj.WithRunPhase(v1alpha1.RunPhasePending)),
				*testobj.Run("default", "apply-1", "apply", testobj.WithWorkspace("workspace-1"), testobj.WithRunPhase(v1alpha1.RunPhasePending)),
			},
			want: []string{"sh-1", "apply-1"},
		},
		{
			name:      "Unapproved privileged command",
			workspace: testobj.Workspace("", "workspace-1", testobj.WithPrivilegedCommands("apply")),
			runs: []v1alpha1.Run{
				*testobj.Run("default", "apply-1", "apply", testobj.WithWorkspace("workspace-1")),
			},
			want: []string(nil),
		},
		{
			name:      "Approved privileged command",
			workspace: testobj.Workspace("", "workspace-1", testobj.WithPrivilegedCommands("apply"), testobj.WithApprovals("apply-1")),
			runs: []v1alpha1.Run{
				*testobj.Run("default", "apply-1", "apply", testobj.WithWorkspace("workspace-1")),
			},
			want: []string{"apply-1"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, updateQueue(tt.workspace, tt.runs))
		})
	}
}
