package command

import (
	"context"
	"fmt"

	"github.com/leg100/stok/crdinfo"
	"github.com/leg100/stok/pkg/apis"
	v1alpha1 "github.com/leg100/stok/pkg/apis/stok/v1alpha1"
	"github.com/leg100/stok/pkg/apis/stok/v1alpha1/command"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

func Add(mgr manager.Manager) error {
	s := mgr.GetScheme()
	apis.AddToScheme(s)

	for crdname, crd := range crdinfo.Inventory {
		o, err := s.New(crd.GroupVersionKind())
		if err != nil {
			return err
		}

		oList, err := s.New(crd.GroupVersionKindList())
		if err != nil {
			return err
		}

		r := &CommandReconciler{
			client:     mgr.GetClient(),
			gvk:        crd.GroupVersionKind(),
			entrypoint: crd.Entrypoint,
			plural:     crd.APIPlural,
			scheme:     s,
		}

		controllerName := fmt.Sprintf("%s-controller", crdname)
		c, err := controller.New(controllerName, mgr, controller.Options{Reconciler: r})
		if err != nil {
			return err
		}

		// Watch for changes to primary command resource
		if err := c.Watch(&source.Kind{Type: o}, &handler.EnqueueRequestForObject{}); err != nil {
			return err
		}

		// Watch for changes to secondary resource Pods and requeue the owner Plan
		err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    o,
		})
		if err != nil {
			return err
		}

		// Watch for changes to resource Workspace and requeue the associated commands
		err = c.Watch(&source.Kind{Type: &v1alpha1.Workspace{}}, &handler.EnqueueRequestsFromMapFunc{
			ToRequests: handler.ToRequestsFunc(func(a handler.MapObject) []reconcile.Request {
				err = r.client.List(context.TODO(), oList, client.InNamespace(a.Meta.GetNamespace()), client.MatchingLabels{
					"workspace": a.Meta.GetName(),
				})
				if err != nil {
					return []reconcile.Request{}
				}

				rr := []reconcile.Request{}
				meta.EachListItem(oList, func(o runtime.Object) error {
					cmd := o.(command.Interface)
					rr = append(rr, reconcile.Request{
						NamespacedName: types.NamespacedName{
							Name:      cmd.GetName(),
							Namespace: cmd.GetNamespace(),
						},
					})
					return nil
				})
				return rr
			}),
		})
		if err != nil {
			return err
		}
	}

	return nil
}
