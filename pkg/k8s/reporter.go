package k8s

import (
	"context"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
)

// Similar to controller-runtime's controller/reconciler, a reporter is wired up to an informer,
// triggering a func on new events for a particular kind. Unlike a reconciler, it's not intended to
// do anything fancy like implement a k8s API.
type Reporter interface {
	// Register creates an informer for a given kind/obj.
	Register(cache.Cache) (cache.Informer, error)

	// MatchingObj tells the cache manager which objects the reporter receives events for.
	// Allows tests to inject objects.
	MatchingObj(interface{}) bool

	// Handler receives events from the informer. Invoked only once.
	Handler(context.Context, <-chan ctrl.Request) error
}
