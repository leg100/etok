package controllers

import (
	"testing"

	"github.com/leg100/etok/api/etok.dev/v1alpha1"
	"github.com/leg100/etok/pkg/testobj"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
)

func TestRunPod(t *testing.T) {
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
			name:      "Terraform workspace",
			run:       testobj.Run("default", "run-12345", "plan"),
			workspace: testobj.Workspace("default", "foo"),
			assertions: func(pod *corev1.Pod) {
				assert.Contains(t, pod.Spec.Containers[0].Env, corev1.EnvVar{
					Name:  "ETOK_NAMESPACE",
					Value: "default",
				})
				assert.Contains(t, pod.Spec.Containers[0].Env, corev1.EnvVar{
					Name:  "ETOK_WORKSPACE",
					Value: "foo",
				})
			},
		},
		{
			name:      "TF_WORKSPACE is set for shell commands",
			run:       testobj.Run("default", "run-12345", "sh"),
			workspace: testobj.Workspace("default", "foo"),
			assertions: func(pod *corev1.Pod) {
				assert.Contains(t, pod.Spec.Containers[0].Env, corev1.EnvVar{
					Name:  "TF_WORKSPACE",
					Value: "default_foo",
				})
			},
		},
		{
			name:      "Terraform binary volume mount",
			run:       testobj.Run("default", "run-12345", "plan"),
			workspace: testobj.Workspace("foo", "bar"),
			assertions: func(pod *corev1.Pod) {
				assert.Contains(t, pod.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
					Name:      "cache",
					MountPath: "/terraform-bins",
					SubPath:   "terraform-bins/",
				})
			},
		},
		{
			name:      ".terraform volume mount",
			run:       testobj.Run("default", "run-12345", "plan", testobj.WithConfigMapPath("subdir")),
			workspace: testobj.Workspace("default", "foo"),
			assertions: func(pod *corev1.Pod) {
				assert.Contains(t, pod.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
					Name:      "cache",
					MountPath: "/workspace/subdir/.terraform",
					SubPath:   ".terraform/",
				})
			},
		},
		{
			name:      "Set environment variables for secrets",
			run:       testobj.Run("default", "run-12345", "plan"),
			workspace: testobj.Workspace("default", "foo", testobj.WithSecret("etok")),
			assertions: func(pod *corev1.Pod) {
				assert.Contains(t, pod.Spec.Containers[0].EnvFrom, corev1.EnvFromSource{
					SecretRef: &corev1.SecretEnvSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "etok",
						},
					},
				})
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.assertions(runPod(tt.run, tt.workspace, "etok:latest"))
		})
	}
}
