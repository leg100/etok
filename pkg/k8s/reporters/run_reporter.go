package reporters

import (
	"context"
	"fmt"
	"time"

	"github.com/apex/log"
	"github.com/leg100/stok/api/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type RunReporter struct {
	client.Client
	Id             string
	EnqueueTimeout time.Duration
	QueueTimeout   time.Duration
}

func (r *RunReporter) Register(c cache.Cache) (cache.Informer, error) {
	return c.GetInformer(context.TODO(), &v1alpha1.Run{})
}

func (r *RunReporter) MatchingObj(obj interface{}) bool {
	_, ok := obj.(*v1alpha1.Run)
	return ok
}

func (r *RunReporter) Handler(ctx context.Context, events <-chan ctrl.Request) error {
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

func (r *RunReporter) report(req ctrl.Request, enqueueTimer, queueTimer *time.Timer) (bool, error) {
	// Ignore event for a different run
	if req.Name != r.Id {
		return false, nil
	}

	log := log.WithField("command", req)

	// Fetch the Run instance
	run := &v1alpha1.Run{}
	if err := r.Get(context.TODO(), req.NamespacedName, run); err != nil {
		if errors.IsNotFound(err) {
			// Run yet to be created
			return false, nil
		}
		// Some error other than not found.
		// TODO: apart from transitory errors, which we could try to recover from.
		return false, err
	}

	// It's no longer pending, so stop pending timer
	if run.GetPhase() != v1alpha1.RunPhasePending {
		enqueueTimer.Stop()

		// And it's no longer queued either, so stop queue timer
		if run.GetPhase() != v1alpha1.RunPhaseQueued {
			queueTimer.Stop()
		}
	}

	// TODO: move to new pods reporter
	log.WithField("phase", run.GetPhase()).Debug("Event received")
	switch run.GetPhase() {
	case v1alpha1.RunPhaseSync:
		// Proceed: pod is now waiting for us to synchronise
		return true, nil
	case v1alpha1.RunPhaseRunning, v1alpha1.RunPhaseCompleted:
		// This should never happen
		return true, fmt.Errorf("command unexpectedly skipped synchronisation")
	default:
		return false, nil
	}
}
