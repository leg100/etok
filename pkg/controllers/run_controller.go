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
	log.V(0).Info("Reconciling")

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
		return ctrl.Result{}, err
	}

	if run.ConfigMap != "" {
		// Fetch its ConfigMap object
		var cm corev1.ConfigMap
		if err := r.Get(ctx, req.NamespacedName, &cm); err != nil {
			return ctrl.Result{}, client.IgnoreNotFound(err)
		} else {
			// Make run owner of configmap (so that if run is deleted, so is its
			// config map)
			if err := controllerutil.SetControllerReference(&run, &cm, r.Scheme); err != nil {
				return ctrl.Result{}, err
			}
		}
	}

	// Check workspace queue position
	position := slice.StringIndex(ws.Status.Queue, run.Name)

	var pod corev1.Pod
	var podCreated bool
	if position == 0 {
		// First position in workspace queue so go ahead and create pod if it
		// doesn't exist yet
		if err := r.Get(ctx, req.NamespacedName, &pod); err != nil {
			if errors.IsNotFound(err) {
				pod = *runner.NewRunPod(&run, &ws, r.Image)

				// Make run owner of pod
				if err := controllerutil.SetControllerReference(&run, &pod, r.Scheme); err != nil {
					return ctrl.Result{}, err
				}

				if err := r.Create(ctx, &pod); err != nil {
					log.Error(err, "unable to create pod")
					return ctrl.Result{}, err
				}
				podCreated = true
			} else {
				return ctrl.Result{}, err
			}
		}
	}

	// Update run phase to reflect pod status
	var phase v1alpha1.RunPhase
	switch {
	case position == 0:
		if podCreated {
			// Brand new pod won't have a status yet
			phase = v1alpha1.RunPhaseProvisioning
		} else {
			switch pod.Status.Phase {
			case corev1.PodSucceeded, corev1.PodFailed:
				phase = v1alpha1.RunPhaseCompleted
			case corev1.PodRunning:
				phase = v1alpha1.RunPhaseRunning
			case corev1.PodPending:
				phase = v1alpha1.RunPhaseProvisioning
			case corev1.PodUnknown:
				return ctrl.Result{}, fmt.Errorf("unknown pod phase")
			default:
				return ctrl.Result{}, fmt.Errorf("unknown pod phase: %s", pod.Status.Phase)
			}
		}
	case position > 0:
		phase = v1alpha1.RunPhaseQueued
	case position < 0:
		// Not yet queued
		phase = v1alpha1.RunPhasePending
	}

	if run.Phase != phase {
		run.RunStatus.Phase = phase
		if err := r.Status().Update(ctx, &run); err != nil {
			log.Error(err, "unable to update phase")
		}
	}

	return ctrl.Result{}, nil
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
		ToRequests: handler.ToRequestsFunc(func(a handler.MapObject) []ctrl.Request {
			runlist := &v1alpha1.RunList{}
			err := r.List(context.TODO(), runlist, client.InNamespace(a.Meta.GetNamespace()), client.MatchingFields{
				"spec.workspace": a.Meta.GetName(),
			})
			if err != nil {
				return []ctrl.Request{}
			}

			rr := []ctrl.Request{}
			meta.EachListItem(runlist, func(o runtime.Object) error {
				run := o.(*v1alpha1.Run)
				rr = append(rr, ctrl.Request{
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
