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
		name                string
		run                 *v1alpha1.Run
		workspace           *v1alpha1.Workspace
		secretFound         bool
		serviceAccountFound bool
		assertions          func(*corev1.Pod)
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
				assert.Contains(t, pod.Spec.Containers[0].Env, corev1.EnvVar{
					Name:  "TF_VAR_namespace",
					Value: "default",
				})
				assert.Contains(t, pod.Spec.Containers[0].Env, corev1.EnvVar{
					Name:  "TF_VAR_workspace",
					Value: "foo",
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
			name:      "variables volume mount",
			run:       testobj.Run("default", "run-12345", "plan", testobj.WithConfigMapPath("subdir")),
			workspace: testobj.Workspace("default", "foo"),
			assertions: func(pod *corev1.Pod) {
				assert.Contains(t, pod.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
					Name:      "variables",
					MountPath: "/workspace/subdir/_etok_variables.tf",
					SubPath:   "_etok_variables.tf",
				})
			},
		},
		{
			name:        "Set environment variables for secrets",
			run:         testobj.Run("default", "run-12345", "plan"),
			workspace:   testobj.Workspace("default", "foo"),
			secretFound: true,
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
		{
			name:      "Set workspace terraform variables",
			run:       testobj.Run("default", "run-12345", "plan"),
			workspace: testobj.Workspace("default", "foo", testobj.WithVariables("foo", "bar")),
			assertions: func(pod *corev1.Pod) {
				assert.Contains(t, pod.Spec.Containers[0].Env, corev1.EnvVar{
					Name:  "TF_VAR_foo",
					Value: "bar",
				})
			},
		},
		{
			name:      "Set workspace environment variables",
			run:       testobj.Run("default", "run-12345", "plan"),
			workspace: testobj.Workspace("default", "foo", testobj.WithEnvironmentVariables("foo", "bar")),
			assertions: func(pod *corev1.Pod) {
				assert.Contains(t, pod.Spec.Containers[0].Env, corev1.EnvVar{
					Name:  "foo",
					Value: "bar",
				})
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.assertions(runPod(tt.run, tt.workspace, tt.secretFound, tt.serviceAccountFound, "etok:latest"))
		})
	}
}
