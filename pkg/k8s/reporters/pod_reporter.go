package reporters

import (
	"context"
	"fmt"

	"github.com/apex/log"
	"github.com/leg100/stok/api/command"
	"github.com/leg100/stok/pkg/k8s"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	runtimecache "sigs.k8s.io/controller-runtime/pkg/cache"
)

type PodReporter struct {
	k8s.Client
	Id  string
	Cmd command.Interface
	Log *log.Entry
}

func (r *PodReporter) Register(cache runtimecache.Cache) (runtimecache.Informer, error) {
	return cache.GetInformer(context.TODO(), &corev1.Pod{})
}

func (r *PodReporter) MatchingObj(obj interface{}) bool {
	_, ok := obj.(*corev1.Pod)
	return ok
}

func (r *PodReporter) Handler(ctx context.Context, events <-chan ctrl.Request) error {
	for {
		select {
		case e := <-events:
			pod := &corev1.Pod{}
			ready, err := r.isRunningAndReady(ctx, e, pod)
			if err != nil {
				return err
			}
			if ready {
				return r.Connect(ctx, pod)
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// Connect obtains a stream of logs from the pod, before attaching to the pod TTY. If the attachment
// fails then it'll fall back to printing the logs. Finally it releases the 'client hold' on the cmd
// obj, informing the runner that it can proceed to running terraform.
func (r *PodReporter) Connect(ctx context.Context, pod *corev1.Pod) error {
	r.Log.Debug("retrieving log stream")
	stream, err := r.GetLogs(pod.GetNamespace(), pod.GetName(), &corev1.PodLogOptions{Follow: true})
	if err != nil {
		return err
	}
	defer stream.Close()

	// Attach to pod tty
	errors := make(chan error)
	go func() {
		r.Log.Debug("attaching")
		errors <- k8s.AttachFallbackToLogs(r.Client, pod, stream)
	}()

	// Let operator know the client is now ready i.e. it's streaming logs and/or attached
	if err := k8s.ReleaseHold(ctx, r.Client, r.Cmd); err != nil {
		return err
	}

	return <-errors
}

func (r *PodReporter) isRunningAndReady(ctx context.Context, req ctrl.Request, pod *corev1.Pod) (bool, error) {
	// Ignore event for a different pod
	if req.Name != r.Id {
		return false, nil
	}

	// Fetch the Pod instance
	if err := r.Get(ctx, req.NamespacedName, pod); err != nil {
		if errors.IsNotFound(err) {
			// Pod not yet created
			return false, nil
		}
		// TODO: recover from transitory errors
		return false, err
	}

	switch pod.Status.Phase {
	case corev1.PodSucceeded:
		return false, ErrPodCompleted
	case corev1.PodFailed:
		return false, fmt.Errorf(pod.Status.ContainerStatuses[0].State.Terminated.Message)
	case corev1.PodRunning:
		conditions := pod.Status.Conditions
		if conditions == nil {
			return false, nil
		}
		for i := range conditions {
			if conditions[i].Type == corev1.PodReady &&
				conditions[i].Status == corev1.ConditionTrue {
				return true, nil
			}
		}
	}
	return false, nil
}

// ErrPodCompleted is returned by PodRunning or PodContainerRunning to indicate that
// the pod has already reached completed state.
var ErrPodCompleted = fmt.Errorf("pod ran to completion")
