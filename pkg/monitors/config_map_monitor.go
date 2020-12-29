package monitors

import (
	"context"

	"github.com/leg100/etok/pkg/handlers"
	"github.com/leg100/etok/pkg/k8s"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	watchtools "k8s.io/client-go/tools/watch"
)

// ConfigMapMonitor waits for the creation of a config map. Non-blocking, the
// config map once found is returned and an error is reported via the returned
// error channel.
func ConfigMapMonitor(ctx context.Context, client kubernetes.Interface, name, namespace string) chan error {
	errch := make(chan error)
	go func() {
		lw := &k8s.ConfigMapListWatcher{Client: client, Name: name, Namespace: namespace}
		_, err := watchtools.UntilWithSync(ctx, lw, &corev1.ConfigMap{}, nil, func(event watch.Event) (bool, error) {
			configmap := event.Object.(*corev1.ConfigMap)

			// ListWatcher field selector filters out other resources but the
			// fake client doesn't implement the field selector, so the
			// following is necessary purely for testing purposes
			if configmap.GetName() != name {
				return false, nil
			}

			if event.Type == watch.Deleted {
				return false, handlers.ErrResourceUnexpectedlyDeleted
			}

			return true, nil
		})

		errch <- err
	}()
	return errch
}
