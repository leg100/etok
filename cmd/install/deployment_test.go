package install

import (
	"testing"

	"github.com/leg100/etok/pkg/backup"
	"github.com/leg100/etok/pkg/testutil"
	"github.com/leg100/etok/pkg/version"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

func TestDeployment(t *testing.T) {
	tests := []struct {
		name       string
		namespace  string
		opts       []podTemplateOption
		assertions func(*appsv1.Deployment)
	}{
		{
			name:      "defaults",
			namespace: "default",
			assertions: func(deploy *appsv1.Deployment) {
				assert.Equal(t, "test-image", deploy.Spec.Template.Spec.Containers[0].Image)
			},
		},
		{
			name:      "with secret",
			namespace: "default",
			opts:      []podTemplateOption{WithSecret(true)},
			assertions: func(deploy *appsv1.Deployment) {
				assert.Contains(t, deploy.Spec.Template.Spec.Containers[0].EnvFrom, corev1.EnvFromSource{
					SecretRef: &corev1.SecretEnvSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "etok",
						},
					},
				})
			},
		},
		{
			name:      "with backup enabled",
			namespace: "default",
			opts:      []podTemplateOption{WithBackupConfig(backup.NewConfig())},
			assertions: func(deploy *appsv1.Deployment) {
				assert.Contains(t, deploy.Spec.Template.Spec.Containers[0].Env, corev1.EnvVar{
					Name:  "ETOK_BACKUP_PROVIDER",
					Value: "",
				})
				assert.Contains(t, deploy.Spec.Template.Spec.Containers[0].Env, corev1.EnvVar{
					Name:  "ETOK_GCS_BUCKET",
					Value: "",
				})
				assert.Contains(t, deploy.Spec.Template.Spec.Containers[0].Env, corev1.EnvVar{
					Name:  "ETOK_S3_BUCKET",
					Value: "",
				})
			},
		},
	}
	for _, tt := range tests {
		testutil.Run(t, tt.name, func(t *testutil.T) {
			t.Override(&version.Image, "test-image")

			tt.assertions(deployment(tt.namespace, tt.opts...))
		})
	}
}
