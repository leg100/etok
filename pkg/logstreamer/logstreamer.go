package logstreamer

import (
	"context"
	"io"

	"github.com/leg100/stok/pkg/log"
	corev1 "k8s.io/api/core/v1"

	typedv1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

// Substitutable for testing
type GetLogsFunc func(context.Context, Options) (io.ReadCloser, error)

type Options struct {
	podsClient    typedv1.PodInterface
	podName       string
	podLogOptions *corev1.PodLogOptions
}

func Stream(ctx context.Context, f GetLogsFunc, out io.Writer, podsClient typedv1.PodInterface, podName, containerName string) error {
	log.Debug("Streaming logs")
	stream, err := f(ctx, Options{
		podsClient:    podsClient,
		podName:       podName,
		podLogOptions: &corev1.PodLogOptions{Follow: true, Container: containerName},
	})
	if err != nil {
		return err
	}
	defer stream.Close()
	_, err = io.Copy(out, stream)
	return err
}

func GetLogs(ctx context.Context, opts Options) (io.ReadCloser, error) {
	return opts.podsClient.GetLogs(opts.podName, opts.podLogOptions).Stream(ctx)
}
