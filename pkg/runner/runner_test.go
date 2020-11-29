package runner

import (
	corev1 "k8s.io/api/core/v1"
)

func ContainsVolumeMount(c corev1.Container, mount corev1.VolumeMount) bool {
	for _, m := range c.VolumeMounts {
		if m == mount {
			return true
		}
	}
	return false
}
