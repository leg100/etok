package k8s

import (
	"context"
	"io"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

var (
	GetLogs = getLogs
)

func getLogs(ctx context.Context, kc kubernetes.Interface, pod *corev1.Pod, container string) (io.ReadCloser, error) {
	return kc.CoreV1().Pods(pod.GetNamespace()).GetLogs(pod.GetName(), &corev1.PodLogOptions{Follow: true, Container: "runner"}).Stream(ctx)
}
