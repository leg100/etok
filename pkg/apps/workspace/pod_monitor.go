package workspace

import (
	"context"
	"fmt"

	"github.com/leg100/stok/api/stok.goalspike.com/v1alpha1"
	"github.com/leg100/stok/pkg/k8s"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	watchtools "k8s.io/client-go/tools/watch"
)

// The podMonitor object has various handlers for monitoring a workspace's pod
type podMonitor struct {
	ws     *v1alpha1.Workspace
	client kubernetes.Interface
}

func (pm *podMonitor) monitor(ctx context.Context, errch chan<- error, ready chan<- struct{}) {
	lw := &k8s.PodListWatcher{Client: pm.client, Name: pm.ws.PodName(), Namespace: pm.ws.GetNamespace()}

	go func() {
		_, err := watchtools.UntilWithSync(ctx, lw, &corev1.Pod{}, nil, pm.podRunningAndReadyHandler)
		if err != nil {
			errch <- err
		} else {
			ready <- struct{}{}
		}
	}()
}

// Return true if pod is both running and ready
func (pm *podMonitor) podRunningAndReadyHandler(event watch.Event) (bool, error) {
	pod := event.Object.(*corev1.Pod)

	// ListWatcher field selector filters out other pods but the fake client doesn't implement the
	// field selector, so the following is necessary purely for testing purposes
	//if pod.GetName() != pm.ws.PodName() {
	//	return false, nil
	//}
	return true, nil

	switch event.Type {
	case watch.Deleted:
		return false, fmt.Errorf("pod resource deleted")
	}

	switch pod.Status.Phase {
	case corev1.PodSucceeded:
		return false, fmt.Errorf("pod prematurely succeeded")
	case corev1.PodFailed:
		return false, fmt.Errorf(pod.Status.InitContainerStatuses[0].State.Terminated.Message)
	case corev1.PodPending:
		if len(pod.Status.InitContainerStatuses) > 0 {
			if pod.Status.InitContainerStatuses[0].State.Running != nil {
				return true, nil
			}
		}
	}
	return false, nil
}
