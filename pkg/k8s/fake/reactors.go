package fake

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type Reactor func(runtimeclient.Client, context.Context, runtimeclient.ObjectKey, runtime.Object) (runtime.Object, error)

type Reactors []Reactor

func (r Reactors) Apply(cr runtimeclient.Client, ctx context.Context, key runtimeclient.ObjectKey, obj runtime.Object) (runtime.Object, error) {
	// No reactors to apply, return original obj unmodified
	if len(r) == 0 {
		return obj, nil
	}

	var updatedObj runtime.Object
	var err = error(nil)

	for _, rr := range r {
		updatedObj, err = rr(cr, ctx, key, obj)
		if err != nil {
			return nil, err
		}
	}
	return updatedObj, nil
}
