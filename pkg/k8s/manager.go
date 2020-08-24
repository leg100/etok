package k8s

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	toolscache "k8s.io/client-go/tools/cache"
	ctrl "sigs.k8s.io/controller-runtime"
	runtimecache "sigs.k8s.io/controller-runtime/pkg/cache"
)

// Wrapper for controller-runtime cache
type Manager struct {
	runtimecache.Cache
	Objs      []runtime.Object
	reporters []ManagedReporter
}

func (m *Manager) AddReporter(r Reporter) error {
	informer, err := r.Register(m.Cache)
	if err != nil {
		return err
	}

	var injectObjs []interface{}
	for _, o := range m.Objs {
		if r.MatchingObj(o) {
			injectObjs = append(injectObjs, o)
		}
	}

	toRequest := func(obj interface{}) ctrl.Request {
		return ctrl.Request{NamespacedName: GetNamespacedName(obj.(metav1.Object))}
	}

	events := make(chan ctrl.Request, len(injectObjs))
	for _, o := range injectObjs {
		events <- toRequest(o)
	}

	informer.AddEventHandler(toolscache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { events <- toRequest(obj) },
		UpdateFunc: func(_, obj interface{}) { events <- toRequest(obj) },
		DeleteFunc: func(obj interface{}) { events <- toRequest(obj) },
	})

	m.reporters = append(m.reporters, ManagedReporter{Reporter: r, events: events})

	return nil
}

// Start handlers and informer cache concurrently. Blocks until either returns.
func (m *Manager) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	err := make(chan error)

	for _, r := range m.reporters {
		go func(r ManagedReporter) {
			err <- r.Handler(ctx, r.events)
		}(r)
	}

	go func() {
		err <- m.Cache.Start(ctx.Done())
	}()

	return <-err
}

type ManagedReporter struct {
	Reporter
	events chan ctrl.Request
}
