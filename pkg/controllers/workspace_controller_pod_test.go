package controllers

import (
	"testing"

	"github.com/leg100/etok/api/etok.dev/v1alpha1"
	"github.com/leg100/etok/pkg/testobj"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
)

func TestWorkspacePod(t *testing.T) {
	tests := []struct {
		name       string
		workspace  *v1alpha1.Workspace
		assertions func(*corev1.Pod)
	}{
		{
			name:      "User specified service account",
			workspace: testobj.Workspace("default", "foo", testobj.WithServiceAccount("bar")),
			assertions: func(pod *corev1.Pod) {
				assert.Equal(t, "bar", pod.Spec.ServiceAccountName)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pod, err := WorkspacePod(tt.workspace, "etok:latest")
			require.NoError(t, err)
			tt.assertions(pod)
		})
	}
}
