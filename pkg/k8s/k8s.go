package k8s

import (
	"context"
	"io"
	"os"

	"github.com/apex/log"
	"github.com/leg100/stok/api"
	"github.com/leg100/stok/api/v1alpha1"
	"k8s.io/client-go/util/retry"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func GetNamespacedName(obj metav1.Object) types.NamespacedName {
	return types.NamespacedName{Name: obj.GetName(), Namespace: obj.GetNamespace()}
}

func ReleaseHold(ctx context.Context, sc Client, obj api.Object) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		key := types.NamespacedName{Name: obj.GetName(), Namespace: obj.GetNamespace()}
		if err := sc.Get(ctx, key, obj); err != nil {
			return err
		}

		// Delete annotation WaitAnnotationKey, giving the runner the signal to start
		annotations := obj.GetAnnotations()
		delete(annotations, v1alpha1.WaitAnnotationKey)
		obj.SetAnnotations(annotations)

		return sc.Update(ctx, obj, &runtimeclient.UpdateOptions{})
	})
}

// Attach to pod, falling back to streaming logs on error
func AttachFallbackToLogs(sc Client, pod *corev1.Pod, logstream io.ReadCloser) error {
	err := sc.Attach(pod)
	if err != nil {
		// TODO: use log fields
		log.Warn("Failed to attach to pod TTY; falling back to streaming logs")
		_, err = io.Copy(os.Stdout, logstream)
		return err
	}
	return nil
}
