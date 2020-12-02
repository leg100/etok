package runner

import (
	"github.com/leg100/etok/api/etok.dev/v1alpha1"
	"github.com/leg100/etok/pkg/labels"
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

	// Set etok's common labels
	labels.SetCommonLabels(pod)
	// Permit filtering pods by workspace
	labels.SetLabel(pod, labels.Workspace(ws.Name))

	return pod
}
