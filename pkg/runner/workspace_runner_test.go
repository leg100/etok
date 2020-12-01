package runner

import (
	"testing"

	"github.com/leg100/stok/api/stok.goalspike.com/v1alpha1"
	"github.com/leg100/stok/pkg/testobj"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
)

func TestWorkspaceRunner(t *testing.T) {
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
			tt.assertions(NewWorkspacePod(tt.workspace, "stok:latest"))
		})
	}
}
