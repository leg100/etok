package controllers

import (
	"context"
	"errors"
	"time"

	v1alpha1 "github.com/leg100/etok/api/etok.dev/v1alpha1"
	"github.com/leg100/etok/cmd/launcher"
	"github.com/leg100/etok/pkg/globals"
	"github.com/leg100/etok/pkg/k8s"
	"github.com/leg100/etok/pkg/scheme"
	"github.com/leg100/etok/pkg/util/slice"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	// runEnqueueTimeout is the maximum time a run can remain waiting to be
	// enqueued
	runEnqueueTimeout = 10 * time.Second
	// runQueueTimeout is the maximum time a run can remain waiting in the queue
	runQueueTimeout = 60 * time.Minute
	// runPodPendingTimeout is the maximum time a pod can remain in the pending
	// phase
	runPodPendingTimeout = 60 * time.Second
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
	runReconcileStatusChain = []runUpdater{}
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

	// Don't reconcile failed or completed runs
	if run.IsDone() {
		return ctrl.Result{}, nil
	}

	// Fetch its Workspace object
	var ws v1alpha1.Workspace
	err := r.Get(ctx, types.NamespacedName{Name: run.Workspace, Namespace: req.Namespace}, &ws)
	if kerrors.IsNotFound(err) {
		// Workspace not found is a fatal event
		meta.SetStatusCondition(&run.RunStatus.Conditions, *runFailed(v1alpha1.WorkspaceNotFoundReason, "Workspace not found"))

		if err := r.updateStatus(ctx, req, run.RunStatus); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	} else if err != nil {
		return ctrl.Result{}, err
	}

	if !isAlreadyOwner(&ws, &run, r.Scheme) {
		// Make workspace owner of run
		if err := controllerutil.SetOwnerReference(&ws, &run, r.Scheme); err != nil {
			return ctrl.Result{}, err
		}
		// ...which adds a field to run metadata, so we need to update
		if err := r.Update(ctx, &run); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Make run owner of configmap archive
	if err := r.setOwnerOfArchive(ctx, &run); err != nil {
		return ctrl.Result{}, err
	}

	// Update status struct
	backoff := processRunReconcileStatusChain(ctx, &run, ws)

	run.Phase = setRunPhase(&run)

	if err := r.updateStatus(ctx, req, run.RunStatus); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, backoff
}

func (r *RunReconciler) updateStatus(ctx context.Context, req ctrl.Request, newStatus v1alpha1.RunStatus) error {
	var run v1alpha1.Run
	if err := r.Get(ctx, req.NamespacedName, &run); err != nil {
		return err
	}

	run.RunStatus = newStatus

	return r.Status().Update(ctx, &run)
}

func processRunReconcileStatusChain(ctx context.Context, run *v1alpha1.Run, ws v1alpha1.Workspace) error {
	for _, f := range runReconcileStatusChain {
		bail, err := f(ctx, run, ws)
		if err != nil || bail {
			return err
		}
	}

	return nil
}

// setRunPhase maps the Run's conditions to a single phase
func setRunPhase(run *v1alpha1.Run) v1alpha1.RunPhase {
	if meta.IsStatusConditionTrue(run.Conditions, v1alpha1.RunFailedCondition) {
		return v1alpha1.RunPhaseFailed
	}

	if meta.IsStatusConditionTrue(run.Conditions, v1alpha1.RunCompleteCondition) {
		return v1alpha1.RunPhaseCompleted
	}

	if meta.IsStatusConditionFalse(run.Conditions, v1alpha1.RunCompleteCondition) {
		switch meta.FindStatusCondition(run.Conditions, v1alpha1.RunCompleteCondition).Reason {
		case v1alpha1.RunUnqueuedReason:
			return v1alpha1.RunPhaseWaiting
		case v1alpha1.RunQueuedReason:
			return v1alpha1.RunPhaseQueued
		case v1alpha1.PodCreatedReason, v1alpha1.PodPendingReason:
			return v1alpha1.RunPhaseProvisioning
		case v1alpha1.PodRunningReason:
			return v1alpha1.RunPhaseRunning
		}
	}

	return v1alpha1.RunPhaseUnknown
}

func (r *RunReconciler) manageQueue(ctx context.Context, run *v1alpha1.Run, ws v1alpha1.Workspace) (bool, error) {
	if !launcher.IsQueueable(run.Command) {
		// Proceed to creating pod
		return false, nil
	}

	if ws.Status.Active == run.Name {
		// Proceed to creating pod
		return false, nil
	}

	var cond *metav1.Condition
	if pos := slice.StringIndex(ws.Status.Queue, run.Name); pos >= 0 {
		cond = runIncomplete(v1alpha1.RunQueuedReason, "Run waiting in workspace queue")
	} else {
		cond = runIncomplete(v1alpha1.RunUnqueuedReason, "Run is waiting to be made active or to be added to workspace queue")
	}
	meta.SetStatusCondition(&run.RunStatus.Conditions, *cond)

	// Fail run if it has exceeded one of two timeouts
	complete := meta.FindStatusCondition(run.RunStatus.Conditions, v1alpha1.RunCompleteCondition)
	lastUpdate := complete.LastTransitionTime.Time

	if complete.Reason == v1alpha1.RunUnqueuedReason {
		if time.Now().After(lastUpdate.Add(runEnqueueTimeout)) {
			// Run has been waiting to be enqueued for too long
			failed := runFailed(v1alpha1.RunEnqueueTimeoutReason, "Timed out waiting to be enqueued")
			meta.SetStatusCondition(&run.RunStatus.Conditions, *failed)
			return true, errors.New("enqueue timeout exceeded")
		}
	}

	if complete.Reason == v1alpha1.RunQueuedReason {
		if time.Now().After(lastUpdate.Add(runQueueTimeout)) {
			// Run has been waiting in queue for too long
			failed := runFailed(v1alpha1.QueueTimeoutReason, "Timed out waiting in the queue")
			meta.SetStatusCondition(&run.RunStatus.Conditions, *failed)
			return true, errors.New("queue wait timeout exceeded")
		}
	}

	// Bail out, do not proceed to creating pod just yet
	return true, nil
}

// Manage run's pod. Update run status to reflect pod status.
func (r *RunReconciler) managePod(ctx context.Context, run *v1alpha1.Run, ws v1alpha1.Workspace) (bool, error) {
	log := log.FromContext(ctx)

	// Check if optional secret "etok" is available
	secretFound := true
	err := r.Get(ctx, types.NamespacedName{Namespace: run.Namespace, Name: "etok"}, &corev1.Secret{})
	if kerrors.IsNotFound(err) {
		secretFound = false
	} else if err != nil {
		return false, err
	}

	// Check if optional service account "etok" is available
	serviceAccountFound := true
	err = r.Get(ctx, types.NamespacedName{Namespace: run.Namespace, Name: "etok"}, &corev1.ServiceAccount{})
	if kerrors.IsNotFound(err) {
		serviceAccountFound = false
	} else if err != nil {
		return false, err
	}

	var pod corev1.Pod
	err = r.Get(ctx, requestFromObject(run).NamespacedName, &pod)
	if kerrors.IsNotFound(err) {
		pod = *runPod(run, &ws, secretFound, serviceAccountFound, r.Image)

		// Make run owner of pod
		if err := controllerutil.SetControllerReference(run, &pod, r.Scheme); err != nil {
			return false, err
		}

		if err := r.Create(ctx, &pod); err != nil {
			log.Error(err, "unable to create pod")
			return false, err
		}
		meta.SetStatusCondition(&run.RunStatus.Conditions, *runIncomplete(v1alpha1.PodCreatedReason, ""))
		return false, nil
	} else if err != nil {
		return false, err
	}

	var isCompleted = metav1.ConditionFalse

	if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
		// Record exit code in run status
		code, err := getExitCode(&pod)
		if err != nil {
			return false, errors.New("unable to retrieve container status")
		}
		run.RunStatus.ExitCode = &code

		isCompleted = metav1.ConditionTrue
	}

	meta.SetStatusCondition(&run.RunStatus.Conditions, metav1.Condition{
		Type:   v1alpha1.RunCompleteCondition,
		Status: isCompleted,
		Reason: getReasonFromPodPhase(pod.Status.Phase),
	})

	// Fail run if pod has been pending too long
	complete := meta.FindStatusCondition(run.RunStatus.Conditions, v1alpha1.RunCompleteCondition)
	if complete.Reason == v1alpha1.PodPendingReason {
		lastUpdate := complete.LastTransitionTime.Time

		if time.Now().After(lastUpdate.Add(runPodPendingTimeout)) {
			failed := runFailed(v1alpha1.RunPendingTimeoutReason, "Timed out waiting for pod in pending phase")
			meta.SetStatusCondition(&run.RunStatus.Conditions, *failed)
		}
	}

	return false, nil
}

// Translate pod phase to a reason string for the run completed condition
func getReasonFromPodPhase(phase corev1.PodPhase) string {
	switch phase {
	case corev1.PodSucceeded:
		return v1alpha1.PodSucceededReason
	case corev1.PodFailed:
		return v1alpha1.PodFailedReason
	case corev1.PodRunning:
		return v1alpha1.PodRunningReason
	case corev1.PodPending:
		return v1alpha1.PodPendingReason
	default:
		return v1alpha1.PodUnknownReason
	}
}

func getExitCode(pod *corev1.Pod) (int, error) {
	status := k8s.ContainerStatusByName(pod, globals.RunnerContainerName)
	if status == nil {
		return 0, errors.New("unable to retrieve container status")
	}
	return int(status.State.Terminated.ExitCode), nil
}

func (r *RunReconciler) setOwnerOfArchive(ctx context.Context, run *v1alpha1.Run) error {
	log := log.FromContext(ctx)

	var archive corev1.ConfigMap
	if err := r.Get(ctx, requestFromObject(run).NamespacedName, &archive); err != nil {
		// Ignore not found errors and keep on reconciling - the client might
		// not yet have created the config map
		if !kerrors.IsNotFound(err) {
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
			// Skip triggering reconcile of runs that are done
			if run.IsDone() {
				continue
			}

			requests = append(requests, requestFromObject(&run))
		}
		return
	}))

	return blder.Complete(r)
}
