package runner

import (
	"context"
	"fmt"

	stokerrors "github.com/leg100/stok/pkg/errors"
	"github.com/leg100/stok/pkg/globals"
	"github.com/leg100/stok/pkg/k8s"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	watchtools "k8s.io/client-go/tools/watch"
)

// Wait for the pod to complete and propagate its error, it has one. The error implements
// errors.ExitError if there is an error, which contains the non-zero exit code of the container.
// Non-blocking, the error is reported via the returned error channel.
func ExitMonitor(ctx context.Context, client kubernetes.Interface, name, namespace string) chan error {
	var code int
	exit := make(chan error)
	go func() {
		lw := &k8s.PodListWatcher{Client: client, Name: name, Namespace: namespace}
		_, err := watchtools.UntilWithSync(ctx, lw, &corev1.Pod{}, nil, func(event watch.Event) (bool, error) {
			pod := event.Object.(*corev1.Pod)

			// ListWatcher field selector filters out other pods but the fake client doesn't implement the
			// field selector, so the following is necessary purely for testing purposes
			if pod.GetName() != name {
				return false, nil
			}

			if event.Type == watch.Deleted {
				return false, fmt.Errorf("pod was unexpectedly deleted")
			}

			if status := k8s.ContainerStatusByName(pod, globals.RunnerContainerName); status != nil {
				if status.State.Terminated != nil {
					code = int(status.State.Terminated.ExitCode)
					return true, nil
				}
			}
			return false, nil
		})

		if err != nil {
			exit <- fmt.Errorf("failed to retrieve exit code: %w", err)
		} else if code != 0 {
			exit <- stokerrors.NewExitError(code)
		} else {
			exit <- nil
		}
	}()
	return exit
}
