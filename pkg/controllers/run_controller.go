package controllers

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	v1alpha1 "github.com/leg100/etok/api/etok.dev/v1alpha1"
	"github.com/leg100/etok/pkg/runner"
	"github.com/leg100/etok/pkg/scheme"
	"github.com/leg100/etok/pkg/util/slice"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
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

// +kubebuilder:rbac:groups=etok.dev,resources=runs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=etok.dev,resources=runs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;create;update;patch;delete

func (r *RunReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("run", req.NamespacedName)
	log.V(0).Info("Reconciling Run")

	// Get run obj
	var run v1alpha1.Run
	if err := r.Get(ctx, req.NamespacedName, &run); err != nil {
		log.Error(err, "unable to get run")
		// we'll ignore not-found errors, since they can't be fixed by an
		// immediate requeue (we'll need to wait for a new notification), and we
		// can get them on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Run completed, nothing more to be done
	if run.Phase == v1alpha1.RunPhaseCompleted {
		return ctrl.Result{}, nil
	}

	// Fetch its Workspace object
	var ws v1alpha1.Workspace
	if err := r.Get(ctx, types.NamespacedName{Name: run.Workspace, Namespace: req.Namespace}, &ws); err != nil {
		return ctrl.Result{}, err
	}

	// Make workspace owner of run (so that if workspace is deleted, so are its runs)
	if err := controllerutil.SetControllerReference(&ws, &run, r.Scheme); err != nil {
		return reconcile.Result{}, err
	}

	// Check workspace queue position
	pos := slice.StringIndex(ws.Status.Queue, run.Name)
	switch {
	case pos == 0:
		// Currently scheduled to run; get or create pod
		return r.reconcilePod(ctx, req, &run, &ws)
	case pos > 0:
		run.Phase = v1alpha1.RunPhaseQueued
	case pos < 0:
		// Not yet queued
		run.Phase = v1alpha1.RunPhasePending
	}

	return reconcile.Result{}, r.Update(ctx, &run)
}

func (r *RunReconciler) reconcilePod(ctx context.Context, request reconcile.Request, run *v1alpha1.Run, ws *v1alpha1.Workspace) (reconcile.Result, error) {
	// Check if pod exists already
	var pod corev1.Pod
	err := r.Get(ctx, request.NamespacedName, &pod)
	if errors.IsNotFound(err) {
		return r.createPod(ctx, run, ws)
	}
	if err != nil {
		return reconcile.Result{}, err
	}

	return r.updateStatus(ctx, &pod, run, ws)
}

func (r *RunReconciler) updateStatus(ctx context.Context, pod *corev1.Pod, run *v1alpha1.Run, ws *v1alpha1.Workspace) (reconcile.Result, error) {
	// Signal pod completion to workspace
	switch pod.Status.Phase {
	case corev1.PodSucceeded, corev1.PodFailed:
		run.Phase = v1alpha1.RunPhaseCompleted
	case corev1.PodRunning:
		run.Phase = v1alpha1.RunPhaseRunning
	case corev1.PodPending:
		return reconcile.Result{Requeue: true}, nil
	case corev1.PodUnknown:
		return reconcile.Result{}, fmt.Errorf("State of pod could not be obtained")
	default:
		return reconcile.Result{}, fmt.Errorf("Unknown pod phase: %s", pod.Status.Phase)
	}

	err := r.Status().Update(ctx, run)
	return reconcile.Result{}, err
}

func (r RunReconciler) createPod(ctx context.Context, run *v1alpha1.Run, ws *v1alpha1.Workspace) (reconcile.Result, error) {
	pod := runner.NewRunPod(run, ws, r.Image)

	// Set Run instance as the owner and controller
	if err := controllerutil.SetControllerReference(run, pod, r.Scheme); err != nil {
		return reconcile.Result{}, err
	}

	err := r.Create(ctx, pod)
	// ignore error wherein two reconciles in quick succession try to create obj
	if errors.IsAlreadyExists(err) {
		return reconcile.Result{}, nil
	}
	if err != nil {
		return reconcile.Result{}, err
	}

	run.Phase = v1alpha1.RunPhaseProvisioning
	if err := r.Status().Update(ctx, run); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{Requeue: true}, nil
}

func (r *RunReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Watch changes to primary run resources
	blder := ctrl.NewControllerManagedBy(mgr).For(&v1alpha1.Run{})

	// Watch for changes to secondary resource Pods and requeue the owner run resource
	blder = blder.Owns(&corev1.Pod{})

	// Index field Spec.Workspace in order for the filtered watch below to work
	_ = mgr.GetFieldIndexer().IndexField(context.TODO(), &v1alpha1.Run{}, "spec.workspace", func(o runtime.Object) []string {
		ws := o.(*v1alpha1.Run).Workspace
		if ws == "" {
			return nil
		}
		return []string{ws}
	})

	// Watch for changes to resource Workspace and requeue the associated runs
	blder = blder.Watches(&source.Kind{Type: &v1alpha1.Workspace{}}, &handler.EnqueueRequestsFromMapFunc{
		ToRequests: handler.ToRequestsFunc(func(a handler.MapObject) []reconcile.Request {
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
