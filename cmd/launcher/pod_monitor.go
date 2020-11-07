package launcher

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

// The podMonitor object has various handlers for monitoring a run's pod
type podMonitor struct {
	run       *v1alpha1.Run
	client    kubernetes.Interface
	attaching bool
}

func (pm *podMonitor) monitor(ctx context.Context, pod chan<- *corev1.Pod, errch chan<- error) {
	lw := &k8s.PodListWatcher{Client: pm.client, Name: pm.run.GetName(), Namespace: pm.run.GetNamespace()}

	go func() {
		event, err := watchtools.UntilWithSync(ctx, lw, &corev1.Pod{}, nil, pm.podRunningAndReadyHandler)
		if err != nil {
			errch <- err
		}
		pod <- event.Object.(*corev1.Pod)
	}()
}

// Return true if pod is both ready and running
func (pm *podMonitor) podRunningAndReadyHandler(event watch.Event) (bool, error) {
	pod := event.Object.(*corev1.Pod)

	// ListWatcher field selector filters out other pods but the fake client doesn't implement the
	// field selector, so the following is necessary purely for testing purposes
	if pod.GetName() != pm.run.GetName() {
		return false, nil
	}

	switch event.Type {
	case watch.Deleted:
		return false, fmt.Errorf("pod resource deleted")
	}

	// If attaching to pod, then it needs to be running; otherwise completed is ok (because its logs can
	// still be obtained).
	switch pod.Status.Phase {
	case corev1.PodSucceeded:
		if pm.attaching {
			return false, fmt.Errorf("pod prematurely succeeded")
		} else {
			return true, nil
		}
	case corev1.PodFailed:
		if pm.attaching {
			return false, fmt.Errorf(pod.Status.ContainerStatuses[0].State.Terminated.Message)
		} else {
			return true, nil
		}
	case corev1.PodRunning:
		if pod.Status.Conditions == nil {
			return false, nil
		}
		for _, cond := range pod.Status.Conditions {
			if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
				return true, nil
			}
		}
	}
	return false, nil
}
