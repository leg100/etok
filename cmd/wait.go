package cmd

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/apex/log"
	"github.com/leg100/stok/pkg/controller/command"
	"github.com/operator-framework/operator-sdk/pkg/status"
	"github.com/spf13/viper"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	watchtools "k8s.io/client-go/tools/watch"
	"k8s.io/kubectl/pkg/util/interrupt"
)

// waitForPod watches the given pod until the exitCondition is true
func (app *app) waitForPod(name string, exitCondition watchtools.ConditionFunc, timeout time.Duration) (*corev1.Pod, error) {
	ctx, cancel := watchtools.ContextWithOptionalTimeout(context.Background(), timeout)
	defer cancel()

	fieldSelector := fields.OneTermEqualSelector("metadata.name", name).String()
	lw := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			options.FieldSelector = fieldSelector
			return app.kubeClient.CoreV1().Pods(viper.GetString("namespace")).List(options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			options.FieldSelector = fieldSelector
			return app.kubeClient.CoreV1().Pods(viper.GetString("namespace")).Watch(options)
		},
	}

	intr := interrupt.New(nil, cancel)
	var result *corev1.Pod
	err := intr.Run(func() error {
		ev, err := watchtools.UntilWithSync(ctx, lw, &corev1.Pod{}, nil, func(ev watch.Event) (bool, error) {
			return exitCondition(ev)
		})
		if ev != nil {
			result = ev.Object.(*corev1.Pod)
		}
		return err
	})

	return result, err
}

// waitForWorkspaceReady watches the command until the WorkspaceReady condition is true
func (app *app) waitForWorkspaceReady(cmd command.Command, timeout time.Duration) (command.Command, error) {
	ctx, cancel := watchtools.ContextWithOptionalTimeout(context.Background(), timeout)
	defer cancel()

	fieldSelector := fields.OneTermEqualSelector("metadata.name", cmd.GetName())
	lw := cache.NewListWatchFromClient(app.clientset.RESTClient(), app.crd.APIPlural, viper.GetString("namespace"), fieldSelector)

	intr := interrupt.New(nil, cancel)
	var result command.Command
	err := intr.Run(func() error {
		ev, err := watchtools.UntilWithSync(ctx, lw, cmd, nil, func(ev watch.Event) (bool, error) {
			switch ev.Type {
			case watch.Deleted:
				return false, errors.NewNotFound(schema.GroupResource{Resource: app.crd.APIPlural}, "")
			}
			if t, ok := ev.Object.(command.Command); ok {
				condition := t.GetConditions().GetCondition(status.ConditionType("WorkspaceReady"))
				if condition != nil {
					status, err := strconv.ParseBool(string(condition.Status))
					if err != nil {
						return false, fmt.Errorf("Could not parse WorkspaceReady condition status")
					}

					log.Debugf("reason: %s", string(condition.Reason))
					switch string(condition.Reason) {
					case "WorkspaceNotFound", "SecretNotFound":
						return false, fmt.Errorf(condition.Message)
					default:
						log.WithFields(log.Fields{
							"msg": condition.Message,
						}).Info("Status update")
						return status, nil
					}
				}
			}

			return false, nil
		})
		if ev != nil {
			result = ev.Object.(command.Command)
		}
		return err
	})

	return result, err
}

// ErrPodCompleted is returned by PodRunning or PodContainerRunning to indicate that
// the pod has already reached completed state.
var ErrPodCompleted = fmt.Errorf("pod ran to completion")

// podRunningAndReady returns true if the pod is running and ready, false if the pod has not
// yet reached those states, returns ErrPodCompleted if the pod has run to completion, or
// an error in any other case.
func podRunningAndReady(event watch.Event) (bool, error) {
	switch event.Type {
	case watch.Deleted:
		return false, errors.NewNotFound(schema.GroupResource{Resource: "pods"}, "")
	}
	switch t := event.Object.(type) {
	case *corev1.Pod:
		switch t.Status.Phase {
		case corev1.PodSucceeded:
			return false, ErrPodCompleted
		case corev1.PodFailed:
			return false, fmt.Errorf(t.Status.ContainerStatuses[0].State.Terminated.Message)
		case corev1.PodRunning:
			conditions := t.Status.Conditions
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
	}
	return false, nil
}
