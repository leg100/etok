package k8s

import corev1 "k8s.io/api/core/v1"

// ContainerStatusByName returns the ContainerStatus object for a container with
// a given name on the pod. Includes init containers.
func ContainerStatusByName(pod *corev1.Pod, name string) *corev1.ContainerStatus {
	allContainerStatuses := append(pod.Status.InitContainerStatuses, pod.Status.ContainerStatuses...)
	for _, status := range allContainerStatuses {
		if status.Name == name {
			return &status
		}
	}
	return nil
}
