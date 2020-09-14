package reporters

import (
	"context"

	"github.com/apex/log"
	"github.com/leg100/stok/api/stok.goalspike.com/v1alpha1"
	"github.com/leg100/stok/pkg/k8s"
	"github.com/leg100/stok/util/slice"
	ctrl "sigs.k8s.io/controller-runtime"
	runtimecache "sigs.k8s.io/controller-runtime/pkg/cache"
)

type WorkspaceReporter struct {
	k8s.Client
	Id    string
	CmdId string
}

func (r *WorkspaceReporter) Register(cache runtimecache.Cache) (runtimecache.Informer, error) {
	return cache.GetInformer(context.TODO(), &v1alpha1.Workspace{})
}

func (r *WorkspaceReporter) MatchingObj(obj interface{}) bool {
	_, ok := obj.(*v1alpha1.Workspace)
	return ok
}

func (r *WorkspaceReporter) Handler(ctx context.Context, events <-chan ctrl.Request) error {
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

func (r *WorkspaceReporter) report(ctx context.Context, req ctrl.Request) (bool, error) {
	// Ignore event for a different ws
	if req.Name != r.Id {
		return false, nil
	}

	wslog := log.WithField("workspace", req)

	// Fetch the Workspace instance
	ws := &v1alpha1.Workspace{}
	if err := r.Get(ctx, req.NamespacedName, ws); err != nil {
		// TODO: recover from transitory errors
		return false, err
	}

	// Report on queue position
	if pos := slice.StringIndex(ws.Status.Queue, r.CmdId); pos >= 0 {
		// TODO: print cmdname in bold
		wslog.WithField("queue", ws.Status.Queue).Info("Queued")
	}

	return false, nil
}
