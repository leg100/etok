package testobj

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func PVC(namespace, name string, opts ...func(*corev1.PersistentVolumeClaim)) *corev1.PersistentVolumeClaim {
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}

	for _, option := range opts {
		option(pvc)
	}

	return pvc
}

func WithPVCPhase(phase corev1.PersistentVolumeClaimPhase) func(*corev1.PersistentVolumeClaim) {
	return func(pvc *corev1.PersistentVolumeClaim) {
		// Only set a phase if non-empty
		if phase != "" {
			pvc.Status.Phase = phase
		}
	}
}
