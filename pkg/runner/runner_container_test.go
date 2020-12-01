package runner

import (
	"testing"

	"github.com/leg100/stok/api/stok.goalspike.com/v1alpha1"
	"github.com/leg100/stok/pkg/testobj"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
)

func TestContainer(t *testing.T) {
	tests := []struct {
		name       string
		runner     Runner
		workspace  *v1alpha1.Workspace
		assertions func(corev1.Container)
	}{
		{
			name:      "Local backend",
			runner:    &run{testobj.Run("default", "run-12345", "plan")},
			workspace: testobj.Workspace("foo", "bar", testobj.WithBackendType("local")),
			assertions: func(c corev1.Container) {
				// Default local backend needs local state directory
				assert.Contains(t, c.VolumeMounts, corev1.VolumeMount{
					Name:      "cache",
					MountPath: "/workspace/terraform.tfstate.d",
					SubPath:   "terraform.tfstate.d/",
				})
			},
		},
		{
			name:      "Remote backend",
			runner:    &run{testobj.Run("default", "run-12345", "plan")},
			workspace: testobj.Workspace("foo", "bar", testobj.WithBackendType("gcs")),
			assertions: func(c corev1.Container) {
				// Remote backends do not have a local state directory
				assert.NotContains(t, c.VolumeMounts, corev1.VolumeMount{
					Name:      "cache",
					MountPath: "/workspace/terraform.tfstate.d",
					SubPath:   "terraform.tfstate.d/",
				})
			},
		},
		{
			name:      "Custom terraform version",
			runner:    &run{testobj.Run("default", "run-12345", "plan")},
			workspace: testobj.Workspace("foo", "bar", testobj.WithTerraformVersion("0.12.17")),
			assertions: func(c corev1.Container) {
				// Specifying a custom terraform version creates a dedicated
				// volume mount for caching the bin when it is downloaded
				assert.Contains(t, c.VolumeMounts, corev1.VolumeMount{
					Name:      "cache",
					MountPath: "/terraform-bins",
					SubPath:   "terraform-bins/",
				})
			},
		},
		{
			name:      "Set environment variables for secrets",
			runner:    &run{testobj.Run("default", "run-12345", "plan")},
			workspace: testobj.Workspace("default", "foo", testobj.WithSecret("stok")),
			assertions: func(c corev1.Container) {
				assert.Contains(t, c.EnvFrom, corev1.EnvFromSource{
					SecretRef: &corev1.SecretEnvSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "stok",
						},
					},
				})
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.assertions(Container(tt.runner, tt.workspace, "stok:latest"))
		})
	}
}
