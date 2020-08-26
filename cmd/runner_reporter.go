package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/leg100/stok/api"
	"github.com/leg100/stok/api/command"
	"github.com/leg100/stok/api/v1alpha1"
	"github.com/leg100/stok/controllers"
	"github.com/leg100/stok/pkg/k8s"
	"github.com/leg100/stok/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
)

type RunnerReporter struct {
	k8s.Client
	name    string
	kind    string
	timeout time.Duration
}

func (r *RunnerReporter) Register(c cache.Cache) (cache.Informer, error) {
	return c.GetInformerForKind(context.TODO(), v1alpha1.SchemeGroupVersion.WithKind(r.kind))
}

func (r *RunnerReporter) MatchingObj(obj interface{}) bool {
	_, ok := obj.(command.Interface)
	return ok
}

func (r *RunnerReporter) Handler(ctx context.Context, events <-chan ctrl.Request) error {
	timer := time.NewTimer(r.timeout)

	for {
		select {
		case e := <-events:
			released, err := r.isReleased(e)
			if err != nil {
				return err
			}
			if released {
				return nil
			}
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			return fmt.Errorf("timeout exceeded waiting for client hold to be released")
		}
	}
}

// isReleased returns true if the client hold has been released on the cmd object; false otherwise
func (r *RunnerReporter) isReleased(req ctrl.Request) (bool, error) {
	// Ignore event for a different cmd
	if req.Name != r.name {
		return false, nil
	}

	// Fetch empty obj using kind
	obj, err := api.NewObjectFromGVK(scheme.Scheme, v1alpha1.SchemeGroupVersion.WithKind(r.kind))
	if err != nil {
		return false, err
	}

	// Populate obj
	if err := r.Get(context.TODO(), req.NamespacedName, obj); err != nil {
		// TODO: recover from transitory errors
		return false, err
	}

	if controllers.IsSynchronising(obj) {
		// Client is yet to synchronise.
		return false, nil
	} else {
		// Client has synchronised, we're clear to proceed.
		return true, nil
	}
}
