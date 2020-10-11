package podhandler

import (
	"context"
	"io"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type Interface interface {
	Attach(*rest.Config, *corev1.Pod, io.Writer) error 
	GetLogs(context.Context, kubernetes.Interface, *corev1.Pod, string) (io.ReadCloser, error)
}
