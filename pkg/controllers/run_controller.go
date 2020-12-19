package controllers

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	v1alpha1 "github.com/leg100/etok/api/etok.dev/v1alpha1"
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

func (r *RunReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
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

	var owned bool
	for _, ref := range run.OwnerReferences {
		if ref.Kind == "Workspace" && ref.Name == ws.Name {
			owned = true
			break
		}
	}
	if !owned {
		// Make workspace owner of run (so that if workspace is deleted, so are
		// its runs)
		if err := controllerutil.SetOwnerReference(&ws, &run, r.Scheme); err != nil {
			return ctrl.Result{}, err
		}
		if err := r.Update(ctx, &run); err != nil {
			log.Error(err, "unable to set workspace owner reference")
		}
	}

	// To be set with current phase
	var phase v1alpha1.RunPhase

	if run.Command != "plan" {
		// Commands other than plan are queued

		// Check workspace queue position
		if pos := slice.StringIndex(ws.Status.Queue, run.Name); pos != 0 {
			// Not at front of queue so update phase and return early
			if pos > 0 {
				phase = v1alpha1.RunPhaseQueued
			} else {
				// Not yet queued
				phase = v1alpha1.RunPhasePending
			}
			if run.Phase != phase {
				run.RunStatus.Phase = phase
				if err := r.Status().Update(ctx, &run); err != nil {
					log.Error(err, "unable to update phase")
					return ctrl.Result{}, err
				}
			}
			// Go no further
			return ctrl.Result{}, nil
		}

		// Front of queue, so continue reconciliation
	}

	// Get or create pod
	var pod corev1.Pod
	var podCreated bool
	if err := r.Get(ctx, req.NamespacedName, &pod); err != nil {
		if errors.IsNotFound(err) {
			pod = *RunPod(&run, &ws, r.Image)

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

	// Update run phase to reflect pod status
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

	if run.Phase != phase {
		run.RunStatus.Phase = phase
		if err := r.Status().Update(ctx, &run); err != nil {
			log.Error(err, "unable to update phase")
			return ctrl.Result{}, err
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
	_ = mgr.GetFieldIndexer().IndexField(context.TODO(), &v1alpha1.Run{}, "spec.workspace", func(o client.Object) []string {
		ws := o.(*v1alpha1.Run).Workspace
		if ws == "" {
			return nil
		}
		return []string{ws}
	})

	// Watch for changes to resource Workspace and requeue the associated runs
	blder = blder.Watches(&source.Kind{Type: &v1alpha1.Workspace{}}, handler.EnqueueRequestsFromMapFunc(func(o client.Object) []ctrl.Request {
		runlist := &v1alpha1.RunList{}
		err := r.List(context.TODO(), runlist, client.InNamespace(o.GetNamespace()), client.MatchingFields{
			"spec.workspace": o.GetName(),
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
	}))

	return blder.Complete(r)
}
