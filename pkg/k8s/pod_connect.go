package k8s

import (
	"context"
	"io"

	"github.com/leg100/stok/pkg/log"
	"github.com/leg100/stok/pkg/podhandler"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// PodConnect does the following:
// 1. Gets log stream of stdout of pod
// 2. Attaches to TTY of pod, failing that it falls back to copying log stream to user stdout
// 3. Releases the 'hold' on the pod, i.e. deletes an annotation, informing the pod runner that it
// can invoke the terraform process, safe in the knowledge that the user is at the very least
// streaming logs
func PodConnect(ctx context.Context, h podhandler.Interface, kc kubernetes.Interface, cfg *rest.Config, pod *corev1.Pod, out io.Writer, release func() error) error {
	log.Debugf("Retrieving log stream for pod %s", GetNamespacedName(pod))
	logstream, err := h.GetLogs(ctx, kc, pod, "runner")
	if err != nil {
		return err
	}
	defer logstream.Close()

	// Attach to pod tty, falling back to log stream upon error
	errors := make(chan error)
	go func() {
		log.Debugf("Attaching to pod %s", GetNamespacedName(pod))
		errors <- attachFallbackToLogs(h, cfg, pod, out, logstream)
	}()

	// Let operator know we're now at least streaming logs (so if there is an error message then at least
	// it'll be fully streamed to the client)
	if err := release(); err != nil {
		return err
	}

	// Wait until attach returns
	return <-errors
}

// Attach to pod, falling back to streaming logs on error
func attachFallbackToLogs(h podhandler.Interface, cfg *rest.Config, pod *corev1.Pod, out io.Writer, logstream io.ReadCloser) error {
	if err := h.Attach(cfg, pod, out); err != nil {
		log.Info("Failed to attach to pod TTY; falling back to streaming logs")
		_, err = io.Copy(out, logstream)
		return err
	}
	return nil
}
