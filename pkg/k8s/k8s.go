package k8s

import (
	"context"
	"time"

	watchtools "k8s.io/client-go/tools/watch"

	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/kubectl/pkg/util/interrupt"
)

// (deprecated) Wrapper for watchtools.UntilWithSync
func WaitUntil(rc rest.Interface, obj runtime.Object, name, namespace, plural string, exitCondition watchtools.ConditionFunc, timeout time.Duration) (runtime.Object, error) {
	fieldSelector := fields.OneTermEqualSelector("metadata.name", name)
	lw := cache.NewListWatchFromClient(rc, plural, namespace, fieldSelector)

	ctx, cancel := watchtools.ContextWithOptionalTimeout(context.Background(), timeout)
	defer cancel()

	intr := interrupt.New(nil, cancel)

	var result runtime.Object
	err := intr.Run(func() error {
		ev, err := watchtools.UntilWithSync(ctx, lw, obj, nil, exitCondition)
		if ev != nil {
			result = ev.Object
		}
		return err
	})
	return result, err
}
