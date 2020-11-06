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
	PodsClient    typedv1.PodInterface
	PodName       string
	PodLogOptions *corev1.PodLogOptions
}

func Stream(ctx context.Context, f GetLogsFunc, out io.Writer, podsClient typedv1.PodInterface, podName, containerName string) error {
	log.Debug("Streaming logs")
	stream, err := f(ctx, Options{
		PodsClient:    podsClient,
		PodName:       podName,
		PodLogOptions: &corev1.PodLogOptions{Follow: true, Container: containerName},
	})
	if err != nil {
		return err
	}
	defer stream.Close()
	_, err = io.Copy(out, stream)
	return err
}

func GetLogs(ctx context.Context, opts Options) (io.ReadCloser, error) {
	return opts.PodsClient.GetLogs(opts.PodName, opts.PodLogOptions).Stream(ctx)
}
