package controllers

import (
	"context"

	"github.com/go-logr/logr"
	v1alpha1 "github.com/leg100/stok/api/stok.goalspike.com/v1alpha1"
	"github.com/leg100/stok/scheme"
	"github.com/leg100/stok/util/slice"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

type RunReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger
	Image  string
}

func NewRunReconciler(c client.Client, image string) *RunReconciler {
	return &RunReconciler{
		Client: c,
		Scheme: scheme.Scheme,
		Log:    ctrl.Log.WithName("controllers").WithName("Run"),
		Image:  image,
	}
}

func (r *RunReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	reqLogger := r.Log.WithValues("run", req.NamespacedName)
	reqLogger.V(0).Info("Reconciling Run")

	// Fetch run obj
	run := &v1alpha1.Run{}
	if err := r.Get(context.TODO(), req.NamespacedName, run); err != nil {
		reqLogger.Error(err, "unable to fetch run obj")

		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Run completed, nothing more to be done
	if run.GetPhase() == v1alpha1.RunPhaseCompleted {
		return ctrl.Result{}, nil
	}

	// Fetch its Workspace object
	ws := &v1alpha1.Workspace{}
	if err := r.Get(context.TODO(), types.NamespacedName{Name: run.GetWorkspace(), Namespace: req.Namespace}, ws); err != nil {
		return ctrl.Result{}, err
	}

	// Make workspace owner of run (so that if workspace is deleted, so are its runs)
	if err := controllerutil.SetControllerReference(ws, run, r.Scheme); err != nil {
		return reconcile.Result{}, err
	}

	// Check workspace queue position
	pos := slice.StringIndex(ws.Status.Queue, run.GetName())
	switch {
	case pos == 0:
		// Currently scheduled to run; get or create pod
		return r.reconcilePod(req, run, ws)
	case pos > 0:
		// Queued
		run.SetPhase(v1alpha1.RunPhaseQueued)
	case pos < 0:
		// Not yet queued
		run.SetPhase(v1alpha1.RunPhasePending)
	}

	return reconcile.Result{}, r.Update(context.TODO(), run)
}

func (r *RunReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Watch changes to primary run resources
	blder := ctrl.NewControllerManagedBy(mgr).For(&v1alpha1.Run{})

	// Watch for changes to secondary resource Pods and requeue the owner run resource
	blder = blder.Owns(&corev1.Pod{})

	// Index field Spec.Workspace in order for the filtered watch below to work (I don't think it
	// works without this...)
	_ = mgr.GetFieldIndexer().IndexField(context.TODO(), &v1alpha1.Run{}, "spec.workspace", func(o runtime.Object) []string {
		ws := o.(*v1alpha1.Run).GetWorkspace()
		if ws == "" {
			return nil
		}
		return []string{ws}
	})

	// Watch for changes to resource Workspace and requeue the associated runs
	blder = blder.Watches(&source.Kind{Type: &v1alpha1.Workspace{}}, &handler.EnqueueRequestsFromMapFunc{
		ToRequests: handler.ToRequestsFunc(func(a handler.MapObject) []reconcile.Request {
			// TODO: MatchingFields will not work....I don't think this is filtering, and so this
			// watch will trigger a reconcile on *all* run resources, not just those belonging to
			// the changed workspace. Instead, we need to enumerate the result of this list and
			// filter.
			runlist := &v1alpha1.RunList{}
			err := r.List(context.TODO(), runlist, client.InNamespace(a.Meta.GetNamespace()), client.MatchingFields{
				"spec.workspace": a.Meta.GetName(),
			})
			if err != nil {
				return []reconcile.Request{}
			}

			rr := []reconcile.Request{}
			meta.EachListItem(runlist, func(o runtime.Object) error {
				run := o.(*v1alpha1.Run)
				rr = append(rr, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      run.GetName(),
						Namespace: run.GetNamespace(),
					},
				})
				return nil
			})
			return rr
		}),
	})

	return blder.Complete(r)
}
