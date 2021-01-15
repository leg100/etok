package controllers

import (
	"context"

	v1alpha1 "github.com/leg100/etok/api/etok.dev/v1alpha1"
	"github.com/leg100/etok/cmd/launcher"
	"github.com/leg100/etok/pkg/scheme"
	"github.com/leg100/etok/pkg/util/slice"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var (
	// List of functions that update the workspace status
	runReconcileStatusChain []runUpdater
)

type runUpdater func(context.Context, *v1alpha1.Run, v1alpha1.Workspace) (bool, error)

type RunReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Image  string
}

func NewRunReconciler(c client.Client, image string) *RunReconciler {
	r := &RunReconciler{
		Client: c,
		Scheme: scheme.Scheme,
		Image:  image,
	}

	// Build chain of status updaters, to be called one after the other in a
	// reconcile
	runReconcileStatusChain = append(runReconcileStatusChain, r.manageQueue)
	runReconcileStatusChain = append(runReconcileStatusChain, r.managePod)

	return r
}

// +kubebuilder:rbac:groups=etok.dev,resources=runs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=etok.dev,resources=runs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;create;update;patch;delete

func (r *RunReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// set up a convenient log object so we don't have to type request over and
	// over again
	log := log.FromContext(ctx)
	log.V(0).Info("Reconciling")

	// Get run obj
	var run v1alpha1.Run
	if err := r.Get(ctx, req.NamespacedName, &run); err != nil {
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

	// Indicate whether a change has been made to the run obj
	var updated bool

	// Make workspace owner of this run
	if err := r.makeWorkspaceOwner(ctx, &run, &ws); err != nil {
		updated = true
		return ctrl.Result{}, err
	}

	// Make run owner of configmap archive
	if err := r.setOwnerOfArchive(ctx, &run); err != nil {
		return ctrl.Result{}, err
	}

	// Update status struct
	reconcileErr := processRunReconcileStatusChain(ctx, &run, ws)

	if updated {
		// Update entire workspace
		if err := r.Update(ctx, &run); err != nil {
			return ctrl.Result{}, err
		}
	} else {
		// Only update workspace status
		if err := r.Status().Update(ctx, &run); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, reconcileErr
}

// Update workspace status.
func processRunReconcileStatusChain(ctx context.Context, run *v1alpha1.Run, ws v1alpha1.Workspace) error {
	for _, f := range runReconcileStatusChain {
		bail, err := f(ctx, run, ws)
		if err != nil || bail {
			// Bail out early
			return err
		}
	}
	return nil
}

func (r *RunReconciler) manageQueue(ctx context.Context, run *v1alpha1.Run, ws v1alpha1.Workspace) (bool, error) {
	if !launcher.IsQueueable(run.Command) {
		return false, nil
	}

	// Check workspace queue position
	pos := slice.StringIndex(ws.Status.Queue, run.Name)
	switch {
	case pos == 0:
		// Front of queue, proceed
		return false, nil
	case pos > 0:
		// Queued, bail out
		run.Phase = v1alpha1.RunPhaseQueued
		return true, nil
	default:
		// Not yet queued, bail out
		run.Phase = v1alpha1.RunPhasePending
		return true, nil
	}
}

// Manage run's pod. Update run status to reflect pod status.
func (r *RunReconciler) managePod(ctx context.Context, run *v1alpha1.Run, ws v1alpha1.Workspace) (bool, error) {
	log := log.FromContext(ctx)

	var pod corev1.Pod
	err := r.Get(ctx, requestFromObject(run).NamespacedName, &pod)
	if errors.IsNotFound(err) {
		pod = *runPod(run, &ws, r.Image)

		// Make run owner of pod
		if err := controllerutil.SetControllerReference(run, &pod, r.Scheme); err != nil {
			return false, err
		}

		if err := r.Create(ctx, &pod); err != nil {
			log.Error(err, "unable to create pod")
			return false, err
		}
		run.Phase = v1alpha1.RunPhaseProvisioning
		return false, nil
	} else if err != nil {
		return false, err
	}

	// Update run phase to reflect pod status
	switch pod.Status.Phase {
	case corev1.PodSucceeded, corev1.PodFailed:
		run.Phase = v1alpha1.RunPhaseCompleted
	case corev1.PodRunning:
		run.Phase = v1alpha1.RunPhaseRunning
	case corev1.PodPending:
		run.Phase = v1alpha1.RunPhaseProvisioning
	default:
		run.Phase = v1alpha1.RunPhaseUnknown
	}

	return false, nil
}

// Make workspace the owner of this run.
func (r *RunReconciler) makeWorkspaceOwner(ctx context.Context, run *v1alpha1.Run, ws *v1alpha1.Workspace) error {
	// Indicate whether run is already owned by workspace or not
	var owned bool
	for _, ref := range run.OwnerReferences {
		if ref.Kind == "Workspace" && ref.Name == ws.Name {
			owned = true
			break
		}
	}
	if !owned {
		if err := controllerutil.SetOwnerReference(ws, run, r.Scheme); err != nil {
			return err
		}
	}
	return nil
}

func (r *RunReconciler) setOwnerOfArchive(ctx context.Context, run *v1alpha1.Run) error {
	log := log.FromContext(ctx)

	var archive corev1.ConfigMap
	if err := r.Get(ctx, requestFromObject(run).NamespacedName, &archive); err != nil {
		// Ignore not found errors and keep on reconciling - the client might
		// not yet have created the config map
		if !errors.IsNotFound(err) {
			log.Error(err, "unable to get archive configmap")
			return err
		}
	} else {
		// Indicate whether archive is already owned by run or not
		var owned bool
		for _, ref := range archive.OwnerReferences {
			if ref.Kind == "Run" && ref.Name == run.Name {
				owned = true
				break
			}
		}
		if !owned {
			if err := controllerutil.SetOwnerReference(run, &archive, r.Scheme); err != nil {
				log.Error(err, "unable to set config map ownership")
				return err
			}
			if err := r.Update(ctx, &archive); err != nil {
				return err
			}
		}
	}
	return nil
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
	blder = blder.Watches(&source.Kind{Type: &v1alpha1.Workspace{}}, handler.EnqueueRequestsFromMapFunc(func(o client.Object) (requests []ctrl.Request) {
		runlist := &v1alpha1.RunList{}
		_ = r.List(context.TODO(), runlist, client.InNamespace(o.GetNamespace()), client.MatchingFields{
			"spec.workspace": o.GetName(),
		})
		for _, run := range runlist.Items {
			// Skip triggering reconcile of completed runs
			if run.Phase != v1alpha1.RunPhaseCompleted {
				requests = append(requests, requestFromObject(&run))
			}
		}
		return
	}))

	return blder.Complete(r)
}
