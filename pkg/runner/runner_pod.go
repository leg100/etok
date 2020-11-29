package runner

import (
	"github.com/leg100/stok/api/stok.goalspike.com/v1alpha1"
	"github.com/leg100/stok/pkg/labels"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Pod(r Runner, ws *v1alpha1.Workspace, image string) *corev1.Pod {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.PodName(),
			Namespace: r.GetNamespace(),
		},
		Spec: corev1.PodSpec{
			RestartPolicy:      corev1.RestartPolicyNever,
			ServiceAccountName: ws.Spec.ServiceAccountName,
			Volumes: []corev1.Volume{
				{
					Name: cacheVolumeName,
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: ws.PVCName(),
						},
					},
				},
				{
					Name: backendConfigVolumeName,
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: v1alpha1.BackendConfigMapName(ws.GetName()),
							},
						},
					},
				},
			},
		},
	}

	// Set stok's common labels
	labels.SetCommonLabels(pod)
	// Permit filtering pods by workspace
	labels.SetLabel(pod, labels.Workspace(ws.Name))

	if ws.Spec.SecretName != "" {
		pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{
			Name: credentialsVolumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: ws.Spec.SecretName,
				},
			},
		})
	}

	return pod
}
