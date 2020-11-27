package runner

import (
	"testing"

	"github.com/leg100/stok/api/stok.goalspike.com/v1alpha1"
	"github.com/leg100/stok/pkg/testobj"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
)

func TestContainer(t *testing.T) {
	tests := []struct {
		name       string
		runner     Runner
		workspace  *v1alpha1.Workspace
		image      string
		assertions func(corev1.Container)
	}{
		{
			name:      "Defaults",
			runner:    testobj.Workspace("foo", "bar"),
			workspace: testobj.Workspace("foo", "bar"),
			assertions: func(container corev1.Container) {
				// Default local backend needs local state directory
				require.True(t, ContainsVolumeMount(container, corev1.VolumeMount{
					Name:      "cache",
					MountPath: "/workspace/terraform.tfstate.d",
					SubPath:   "terraform.tfstate.d/",
				}))
			},
		},
		{
			name:      "Local backend",
			runner:    testobj.Workspace("foo", "bar"),
			workspace: testobj.Workspace("foo", "bar", testobj.WithBackendType("local")),
			assertions: func(container corev1.Container) {
				// Default local backend needs local state directory
				require.True(t, ContainsVolumeMount(container, corev1.VolumeMount{
					Name:      "cache",
					MountPath: "/workspace/terraform.tfstate.d",
					SubPath:   "terraform.tfstate.d/",
				}))
			},
		},
		{
			name:      "Remote backend",
			runner:    testobj.Workspace("foo", "bar"),
			workspace: testobj.Workspace("foo", "bar", testobj.WithBackendType("gcs")),
			assertions: func(container corev1.Container) {
				// Remote backends do not have a local state directory
				require.False(t, ContainsVolumeMount(container, corev1.VolumeMount{
					Name:      "cache",
					MountPath: "/workspace/terraform.tfstate.d",
					SubPath:   "terraform.tfstate.d/",
				}))
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			container := Container(tt.runner, tt.workspace, tt.image)
			tt.assertions(container)
		})
	}
}

func ContainsVolumeMount(c corev1.Container, mount corev1.VolumeMount) bool {
	for _, m := range c.VolumeMounts {
		if m == mount {
			return true
		}
	}
	return false
}
