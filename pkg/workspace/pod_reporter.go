package workspace

import (
	"context"
	"fmt"

	"github.com/leg100/stok/pkg/k8s"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	runtimecache "sigs.k8s.io/controller-runtime/pkg/cache"
)

type WorkspacePodReporter struct {
	k8s.Client
	Id string
}

func (r *WorkspacePodReporter) Register(cache runtimecache.Cache) (runtimecache.Informer, error) {
	return cache.GetInformer(context.TODO(), &corev1.Pod{})
}

func (r *WorkspacePodReporter) MatchingObj(obj interface{}) bool {
	_, ok := obj.(*corev1.Pod)
	return ok
}

func (r *WorkspacePodReporter) Handler(ctx context.Context, events <-chan ctrl.Request) error {
	for {
		select {
		case e := <-events:
			exit, err := r.report(ctx, e)
			if err != nil {
				return err
			}
			if exit {
				return nil
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (r *WorkspacePodReporter) report(ctx context.Context, req ctrl.Request) (bool, error) {
	// Ignore event for a different ws
	if req.Name != r.Id {
		return false, nil
	}

	// Fetch the Workspace instance
	pod := &corev1.Pod{}
	if err := r.Get(ctx, req.NamespacedName, pod); err != nil {
		// TODO: recover from transitory errors
		return false, err
	}

	switch pod.Status.Phase {
	case corev1.PodSucceeded:
		return false, ErrPodCompleted
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

// ErrPodCompleted is returned by PodRunning or PodContainerRunning to indicate that
// the pod has already reached completed state.
var ErrPodCompleted = fmt.Errorf("pod ran to completion")
