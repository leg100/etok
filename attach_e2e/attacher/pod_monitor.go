package main

import (
	"context"
	"fmt"

	"github.com/leg100/stok/pkg/k8s"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	watchtools "k8s.io/client-go/tools/watch"
)

// The podMonitor object has various handlers for monitoring a run's pod
type podMonitor struct {
	namespace, name string
	client          kubernetes.Interface
}

func (pm *podMonitor) monitor(ctx context.Context, errch chan<- error, ready chan<- struct{}) {
	lw := &k8s.PodListWatcher{Client: pm.client, Name: pm.name, Namespace: pm.namespace}

	go func() {
		_, err := watchtools.UntilWithSync(ctx, lw, &corev1.Pod{}, nil, pm.podRunningAndReadyHandler)
		if err != nil {
			errch <- err
		} else {
			ready <- struct{}{}
		}
	}()
}

// Return true if pod is both ready and running
func (pm *podMonitor) podRunningAndReadyHandler(event watch.Event) (bool, error) {
	pod := event.Object.(*corev1.Pod)

	switch event.Type {
	case watch.Deleted:
		return false, fmt.Errorf("pod resource deleted")
	}

	switch pod.Status.Phase {
	case corev1.PodSucceeded:
		return false, fmt.Errorf("pod prematurely succeeded")
	case corev1.PodFailed:
		return false, fmt.Errorf(pod.Status.ContainerStatuses[0].State.Terminated.Message)
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
