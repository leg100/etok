package workspace

import (
	"context"

	"github.com/leg100/stok/api/stok.goalspike.com/v1alpha1"
	"github.com/leg100/stok/pkg/k8s"
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

	// Fetch the Workspace instance
	ws := &v1alpha1.Workspace{}
	if err := r.Get(ctx, req.NamespacedName, ws); err != nil {
		// TODO: recover from transitory errors
		return false, err
	}

	if ws.Status.Conditions.IsTrueFor(v1alpha1.ConditionHealthy) {
		return false, nil
	}

	return false, nil
}
