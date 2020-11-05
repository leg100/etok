package main

import (
	"context"
	"fmt"
	"testing"

	"github.com/leg100/stok/pkg/k8s"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/fake"
	testcore "k8s.io/client-go/testing"
	watchtools "k8s.io/client-go/tools/watch"
)

func TestMain(t *testing.T) {
	watcher := watch.NewFake()
	client := fake.NewSimpleClientset()
	pod := testPod("default", "default")
	client.PrependWatchReactor("pods", testcore.DefaultWatchReactor(watcher, nil))

	lw := &k8s.PodListWatcher{Client: client, Name: pod.GetName(), Namespace: pod.GetNamespace()}

	// Mock run controller
	go func() {
		watcher.Add(pod)
		watcher.Modify(updatePodWithSuccessfulExit(pod))
	}()

	_, err := watchtools.UntilWithSync(context.Background(), lw, &corev1.Pod{}, nil, podRunningAndReadyHandler, podCompleted)
	assert.NoError(t, err)
}

func testPod(namespace, name string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			Conditions: []corev1.PodCondition{
				{
					Type:   corev1.PodReady,
					Status: corev1.ConditionTrue,
				},
			},
		},
	}
}

func updatePodWithSuccessfulExit(pod *corev1.Pod) *corev1.Pod {
	pod.Status.Phase = corev1.PodSucceeded
	pod.Status.ContainerStatuses = []corev1.ContainerStatus{
		{
			State: corev1.ContainerState{
				Terminated: &corev1.ContainerStateTerminated{
					ExitCode: 0,
				},
			},
		},
	}
	return pod
}

// Return true if pod is both ready and running
func podRunningAndReadyHandler(event watch.Event) (bool, error) {
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

// Return true if pod completed
func podCompleted(event watch.Event) (bool, error) {
	pod := event.Object.(*corev1.Pod)

	switch event.Type {
	case watch.Deleted:
		return false, fmt.Errorf("pod resource deleted")
	}

	switch pod.Status.Phase {
	case corev1.PodSucceeded, corev1.PodFailed:
		return true, nil
	default:
		return false, nil
	}
}
