package handlers

import (
	"errors"
	"fmt"

	"github.com/leg100/stok/pkg/k8s"
	"github.com/leg100/stok/pkg/log"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/watch"
	watchtools "k8s.io/client-go/tools/watch"
)

// ContainerReady returns an appropriate event handler for determining when pod
// is ready for attaching to or streaming from.
func ContainerReady(pod, container string, init, isTTY bool) watchtools.ConditionFunc {
	switch {
	case isTTY && init:
		return InitContainerAttachable(pod, container)
	case isTTY:
		return ContainerAttachable(pod, container)
	case init:
		return InitContainerStreamable(pod, container)
	default:
		return ContainerStreamable(pod, container)
	}
}

// InitContainerAttachable is an event handler that returns true if a pod
// initContainer's TTY can be attached to, i.e. the init container has started
// and is ready to handshake (therefore it should not have finished yet).
func InitContainerAttachable(pod, container string) watchtools.ConditionFunc {
	return PodHandlerWrapper(pod, func(pod *corev1.Pod) (bool, error) {
		switch pod.Status.Phase {
		case corev1.PodRunning:
			return false, fmt.Errorf("pod unexpectedly running")
		case corev1.PodSucceeded:
			return false, fmt.Errorf("pod unexpectedly succeeded")
		case corev1.PodFailed:
			return false, fmt.Errorf(k8s.ContainerStatusByName(pod, container).State.Terminated.Message)
		case corev1.PodPending:
			if status := k8s.ContainerStatusByName(pod, container); status != nil {
				if status.State.Running != nil {
					return true, nil
				}
			}
		}
		return false, nil
	})
}

var PrematurelySucceededPodError = errors.New("pod prematurely succeeded")

// ContainerAttachable is an event handler that returns true if a pod
// container's TTY can be attached to, i.e. the container has started
// and is ready to handshake (therefore it should not have finished yet).
func ContainerAttachable(pod, container string) watchtools.ConditionFunc {
	return PodHandlerWrapper(pod, func(pod *corev1.Pod) (bool, error) {
		switch pod.Status.Phase {
		case corev1.PodSucceeded:
			return false, PrematurelySucceededPodError
		case corev1.PodFailed:
			return false, fmt.Errorf(k8s.ContainerStatusByName(pod, container).State.Terminated.Message)
		case corev1.PodRunning:
			return true, nil
		default:
			return false, nil
		}
	})
}

// InitContainerStreamable is an event handler that returns true if the pod
// initContainer's logs can be streamed, i.e. the init container has at least started
func InitContainerStreamable(pod, container string) watchtools.ConditionFunc {
	return PodHandlerWrapper(pod, func(pod *corev1.Pod) (bool, error) {
		switch pod.Status.Phase {
		case corev1.PodSucceeded, corev1.PodFailed, corev1.PodRunning:
			return true, nil
		case corev1.PodPending:
			if status := k8s.ContainerStatusByName(pod, container); status != nil {
				if status.State.Running != nil || status.State.Terminated != nil {
					return true, nil
				}
			}
		}
		return false, nil
	})
}

// ContainerStreamable is an event handler that returns true if the pod
// Container's logs can be streamed, i.e. the container has at least started
func ContainerStreamable(pod, container string) watchtools.ConditionFunc {
	return PodHandlerWrapper(pod, func(pod *corev1.Pod) (bool, error) {
		switch pod.Status.Phase {
		case corev1.PodSucceeded, corev1.PodFailed, corev1.PodRunning:
			return true, nil
		}
		return false, nil
	})
}

type podHandler func(*corev1.Pod) (bool, error)

// PodHandlerWrapper is a wrapper for creating handlers for pod events
func PodHandlerWrapper(name string, h podHandler) watchtools.ConditionFunc {
	// Current pod phase
	var phase corev1.PodPhase = corev1.PodUnknown

	return func(event watch.Event) (bool, error) {
		pod := event.Object.(*corev1.Pod)

		// ListWatcher field selector filters out other pods but the fake client doesn't implement the
		// field selector, so the following is necessary purely for testing purposes
		if pod.Name != name {
			return false, nil
		}

		if event.Type == watch.Deleted {
			return false, fmt.Errorf("pod was unexpectedly deleted")
		}

		if pod.Status.Phase != phase {
			log.Debugf("Pod phase shift: %s -> %s\n", phase, pod.Status.Phase)
			phase = pod.Status.Phase
		}

		return h(pod)
	}
}
