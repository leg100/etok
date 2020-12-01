package runner

import (
	"testing"

	"github.com/leg100/stok/api/stok.goalspike.com/v1alpha1"
	"github.com/leg100/stok/pkg/testobj"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
)

func TestRunRunner(t *testing.T) {
	tests := []struct {
		name       string
		run        *v1alpha1.Run
		workspace  *v1alpha1.Workspace
		assertions func(*corev1.Pod)
	}{
		{
			name:      "Non-default working dir",
			run:       testobj.Run("default", "run-12345", "plan", testobj.WithConfigMapPath("subdir")),
			workspace: testobj.Workspace("default", "foo"),
			assertions: func(pod *corev1.Pod) {
				assert.Equal(t, "/workspace/subdir", pod.Spec.Containers[0].WorkingDir)
			},
		},
		{
			name:      "TF_WORKSPACE",
			run:       testobj.Run("default", "run-12345", "plan"),
			workspace: testobj.Workspace("default", "foo"),
			assertions: func(pod *corev1.Pod) {
				assert.Contains(t, pod.Spec.Containers[0].Env, corev1.EnvVar{
					Name:  "TF_WORKSPACE",
					Value: "default-foo",
				})
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.assertions(NewRunPod(tt.run, tt.workspace, "stok:latest"))
		})
	}
}
