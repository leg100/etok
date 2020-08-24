package reporters

import (
	"context"
	"fmt"
	"time"

	"github.com/leg100/stok/api/command"
	"github.com/leg100/stok/api/v1alpha1"
	"github.com/leg100/stok/pkg/k8s"
	"github.com/leg100/stok/scheme"
	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
)

type CommandReporter struct {
	k8s.Client
	Id             string
	Kind           string
	EnqueueTimeout time.Duration
	QueueTimeout   time.Duration
}

func (r *CommandReporter) Register(c cache.Cache) (cache.Informer, error) {
	return c.GetInformerForKind(context.TODO(), v1alpha1.SchemeGroupVersion.WithKind(r.Kind))
}

func (r *CommandReporter) MatchingObj(obj interface{}) bool {
	_, ok := obj.(command.Interface)
	return ok
}

func (r *CommandReporter) Handler(ctx context.Context, events <-chan ctrl.Request) error {
	enqueueTimer := time.NewTimer(r.EnqueueTimeout)
	queueTimer := time.NewTimer(r.QueueTimeout)

	for {
		select {
		case e := <-events:
			exit, err := r.report(e, enqueueTimer, queueTimer)
			if err != nil {
				return err
			}
			if exit {
				return nil
			}
		case <-enqueueTimer.C:
			return fmt.Errorf("timeout reached for enqueuing command")
		case <-queueTimer.C:
			return fmt.Errorf("timeout reached for queued command")
		}
	}
}

func (r *CommandReporter) report(req ctrl.Request, enqueueTimer, queueTimer *time.Timer) (bool, error) {
	// Ignore event for a different cmd
	if req.Name != r.Id {
		return false, nil
	}

	cmd, err := command.NewCommandFromGVK(scheme.Scheme, v1alpha1.SchemeGroupVersion.WithKind(r.Kind))
	if err != nil {
		return false, err
	}

	// Fetch the Command instance
	if err := r.Get(context.TODO(), req.NamespacedName, cmd); err != nil {
		if errors.IsNotFound(err) {
			// Command yet to be created
			return false, nil
		}
		// Some error other than not found.
		// TODO: apart from transitory errors, which we could try to recover from.
		return false, err
	}

	switch cmd.GetPhase() {
	case v1alpha1.CommandPhaseQueued:
		if !enqueueTimer.Stop() {
			<-enqueueTimer.C
		}
	case v1alpha1.CommandPhaseActive:
		// TODO: need to stop enqueue timer too...
		if !queueTimer.Stop() {
			<-queueTimer.C
		}
		// Command is active, which means the client can now connect to it
		return true, nil
	}

	return false, nil
}
