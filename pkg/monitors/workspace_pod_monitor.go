package monitors

import (
	"context"
	"fmt"

	"github.com/leg100/stok/api/stok.goalspike.com/v1alpha1"
	"github.com/leg100/stok/pkg/k8s"
	"github.com/leg100/stok/pkg/log"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	watchtools "k8s.io/client-go/tools/watch"
)

// The workspacePodMonitor object has various handlers for monitoring a workspace's pod
type workspacePodMonitor struct {
	ws     *v1alpha1.Workspace
	client kubernetes.Interface
	// Whether client will be attaching or streaming logs
	attaching bool

	// Current pod phase
	phase corev1.PodPhase
}

func NewWorkspacePodMonitor(client kubernetes.Interface, ws *v1alpha1.Workspace, attaching bool) *workspacePodMonitor {
	return &workspacePodMonitor{
		client:    client,
		ws:        ws,
		phase:     corev1.PodUnknown,
		attaching: attaching,
	}
}

func (pm *workspacePodMonitor) Monitor(ctx context.Context, errch chan<- error, ready chan<- struct{}) {
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
func (pm *workspacePodMonitor) podRunningAndReadyHandler(event watch.Event) (bool, error) {
	pod := event.Object.(*corev1.Pod)

	// ListWatcher field selector filters out other pods but the fake client doesn't implement the
	// field selector, so the following is necessary purely for testing purposes
	if pod.GetName() != pm.ws.PodName() {
		return false, nil
	}

	switch event.Type {
	case watch.Deleted:
		return false, fmt.Errorf("workspace pod resource deleted")
	}

	if phase := pod.Status.Phase; phase != pm.phase {
		log.Debugf("Pod phase shift: %s -> %s\n", pm.phase, phase)
		pm.phase = phase
	}

	switch pod.Status.Phase {
	case corev1.PodRunning:
		return false, fmt.Errorf("workspace pod unexpectedly running")
	case corev1.PodSucceeded:
		return false, fmt.Errorf("workspace pod unexpectedly succeeded")
	case corev1.PodFailed:
		return false, fmt.Errorf(pod.Status.InitContainerStatuses[0].State.Terminated.Message)
	case corev1.PodPending:
		if len(pod.Status.InitContainerStatuses) > 0 {
			state := pod.Status.InitContainerStatuses[0].State
			if state.Running != nil {
				// Pod is both attachable and streamable
				return true, nil
			}
			if state.Terminated != nil && !pm.attaching {
				// Pod is streamable (but not attachable)
				return true, nil
			}
		}
	}
	return false, nil
}
