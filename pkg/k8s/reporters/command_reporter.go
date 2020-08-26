package reporters

import (
	"context"
	"fmt"
	"time"

	"github.com/apex/log"
	"github.com/leg100/stok/api/command"
	"github.com/leg100/stok/api/v1alpha1"
	"github.com/leg100/stok/scheme"
	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type CommandReporter struct {
	client.Client
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

	log := log.WithField("command", req)

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

	// It's no longer pending, so stop pending timer
	if cmd.GetPhase() != v1alpha1.CommandPhasePending {
		enqueueTimer.Stop()

		// And it's no longer queued either, so stop queue timer
		if cmd.GetPhase() != v1alpha1.CommandPhaseQueued {
			queueTimer.Stop()
		}
	}

	// TODO: move to new pods reporter
	log.WithField("phase", cmd.GetPhase()).Debug("Event received")
	switch cmd.GetPhase() {
	case v1alpha1.CommandPhaseSync:
		// Proceed: pod is now waiting for us to synchronise
		return true, nil
	case v1alpha1.CommandPhaseRunning, v1alpha1.CommandPhaseCompleted:
		// This should never happen
		return true, fmt.Errorf("command unexpectedly skipped synchronisation")
	default:
		return false, nil
	}
}
