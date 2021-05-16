package github

import (
	"context"
	"io"

	"github.com/leg100/etok/pkg/globals"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Streamer for streaming logs from k8s. Abstracted to an interface for testing
// purposes.
type streamer interface {
	Stream(context.Context, client.ObjectKey) (io.ReadCloser, error)
}

type podStreamer struct {
	client kubernetes.Interface
}

func (s *podStreamer) Stream(ctx context.Context, key client.ObjectKey) (io.ReadCloser, error) {
	opts := corev1.PodLogOptions{Container: globals.RunnerContainerName}

	return s.client.CoreV1().Pods(key.Namespace).GetLogs(key.Name, &opts).Stream(ctx)
}
