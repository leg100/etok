package launcher

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/leg100/stok/api/stok.goalspike.com/v1alpha1"
	"github.com/leg100/stok/pkg/k8s"
	"github.com/leg100/stok/pkg/k8s/stokclient"
	"github.com/leg100/stok/pkg/log"
	"github.com/leg100/stok/util/slice"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	watchtools "k8s.io/client-go/tools/watch"
)

// The queueMonitor object has various handlers for monitoring a run's position in a queue
type queueMonitor struct {
	run            *v1alpha1.Run
	workspace      string
	client         stokclient.Interface
	timeoutEnqueue time.Duration
	timeoutQueue   time.Duration
}

func (qm *queueMonitor) monitor(ctx context.Context, errch chan<- error) {
	lw := &k8s.WorkspaceListWatcher{Client: qm.client, Name: qm.workspace, Namespace: qm.run.GetNamespace()}

	// Log queue position
	go func() {
		// Should never return unless context cancelled
		if _, err := watchtools.UntilWithSync(ctx, lw, &v1alpha1.Workspace{}, nil, qm.loggingHandler); err != nil {
			errch <- err
		}
	}()

	// Ensure queuing timeouts are not exceeded
	go func() {
		// Ensure run is queued within enqueue timeout
		ctx, cancel := watchtools.ContextWithOptionalTimeout(ctx, qm.timeoutEnqueue)
		defer cancel()

		ev, err := watchtools.UntilWithSync(ctx, lw, &v1alpha1.Workspace{}, nil, qm.isQueuedHandler)
		if err != nil {
			if errors.Is(err, wait.ErrWaitTimeout) {
				err = fmt.Errorf("timed out waiting for run to be added to workspace queue")
			}
			errch <- err
			return
		}
		ws := ev.Object.(*v1alpha1.Workspace)

		log.Debug("run enqueued within enqueue timeout")

		if slice.StringIndex(ws.Status.Queue, qm.run.GetName()) > 0 {
			// Run is behind at least one other run; ensure run reaches first place within queue
			// timeout
			ctx, cancel := watchtools.ContextWithOptionalTimeout(ctx, qm.timeoutQueue)
			defer cancel()

			_, err := watchtools.UntilWithSync(ctx, lw, &v1alpha1.Workspace{}, nil, qm.isFirstPlaceHandler)
			if err != nil {
				if errors.Is(err, wait.ErrWaitTimeout) {
					err = fmt.Errorf("timed out waiting for run to reach first place in workspace queue")
				}
				errch <- err
			}
			log.Debug("run is in first place within queue timeout")
		}
	}()

}

// Log queue position - purely informative, never exits
func (qm *queueMonitor) loggingHandler(event watch.Event) (bool, error) {
	return wsHandler(event, func(ws *v1alpha1.Workspace) (bool, error) {
		// Report on queue position
		if pos := slice.StringIndex(ws.Status.Queue, qm.run.GetName()); pos >= 0 {
			// TODO: print current run in bold
			log.Infof("Queued: %v", ws.Status.Queue)
		}
		return false, nil
	})
}

// Return true if run is queued
func (qm *queueMonitor) isQueuedHandler(event watch.Event) (bool, error) {
	return wsHandler(event, func(ws *v1alpha1.Workspace) (bool, error) {
		if slice.ContainsString(ws.Status.Queue, qm.run.GetName()) {
			return true, nil
		}
		return false, nil
	})
}

// Return true if run is in position 0
func (qm *queueMonitor) isFirstPlaceHandler(event watch.Event) (bool, error) {
	return wsHandler(event, func(ws *v1alpha1.Workspace) (bool, error) {
		if slice.StringIndex(ws.Status.Queue, qm.run.GetName()) == 0 {
			return true, nil
		}
		return false, nil
	})
}

type WorkspaceHandler func(*v1alpha1.Workspace) (bool, error)

// Event handler wrapper for workspace object events
func wsHandler(event watch.Event, handler WorkspaceHandler) (bool, error) {
	switch event.Type {
	case watch.Deleted:
		return false, fmt.Errorf("workspace resource deleted")
	}

	switch ws := event.Object.(type) {
	case *v1alpha1.Workspace:
		handler(ws)
	}

	return false, nil
}
