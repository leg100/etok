package controllers

import (
	"context"
	"errors"
	"reflect"
	"strings"

	"github.com/leg100/etok/api/etok.dev/v1alpha1"

	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/leg100/etok/pkg/backup"
	"github.com/leg100/etok/pkg/scheme"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	// Names given to RBAC resources. Only one of each is created in any given
	// namespace.
	ServiceAccountName = "etok"
	RoleName           = "etok"
	RoleBindingName    = "etok"
)

var (
	// List of functions that update the workspace status
	workspaceReconcileStatusChain []workspaceUpdater
)

type workspaceUpdater func(context.Context, *v1alpha1.Workspace) (*metav1.Condition, error)

type WorkspaceReconciler struct {
	client.Client
	Scheme         *runtime.Scheme
	Image          string
	recorder       record.EventRecorder
	BackupProvider backup.Provider
}

type WorkspaceReconcilerOption func(r *WorkspaceReconciler)

func WithBackupProvider(bp backup.Provider) WorkspaceReconcilerOption {
	return func(r *WorkspaceReconciler) {
		r.BackupProvider = bp
	}
}

func WithEventRecorder(recorder record.EventRecorder) WorkspaceReconcilerOption {
	return func(r *WorkspaceReconciler) {
		r.recorder = recorder
	}
}

func NewWorkspaceReconciler(cl client.Client, image string, opts ...WorkspaceReconcilerOption) *WorkspaceReconciler {
	r := &WorkspaceReconciler{
		Client: cl,
		Scheme: scheme.Scheme,
		Image:  image,
	}

	for _, o := range opts {
		o(r)
	}

	// Build chain of workspace status updaters, to be called one after the
	// other in a reconcile
	workspaceReconcileStatusChain = []workspaceUpdater{}
	workspaceReconcileStatusChain = append(workspaceReconcileStatusChain, r.handleDeletion)
	workspaceReconcileStatusChain = append(workspaceReconcileStatusChain, r.manageQueue)
	workspaceReconcileStatusChain = append(workspaceReconcileStatusChain, r.manageBuiltins)
	workspaceReconcileStatusChain = append(workspaceReconcileStatusChain, r.manageRBACForNamespace)
	workspaceReconcileStatusChain = append(workspaceReconcileStatusChain, r.manageState)
	workspaceReconcileStatusChain = append(workspaceReconcileStatusChain, r.managePVC)
	workspaceReconcileStatusChain = append(workspaceReconcileStatusChain, r.managePod)

	return r
}

// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=roles,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=rolebindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// Manage configmaps for terraform variables
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete

// Read terraform state files
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

// Operator grants these permissions to workspace service accounts, therefore it
// too needs these permissions.
// +kubebuilder:rbac:groups="etok.dev",resources=runs,verbs=get
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=create
// +kubebuilder:rbac:groups="coordination.k8s.io",resources=leases,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete

// for metrics:
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get
// +kubebuilder:rbac:groups=apps,resources=replicasets,verbs=get

// +kubebuilder:rbac:groups=etok.dev,resources=workspaces,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=etok.dev,resources=workspaces/status,verbs=get;update;patch

// Reconcile reads that state of the cluster for a Workspace object and makes changes based on the state read
// and what is in the Workspace.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *WorkspaceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// set up a convenient log object so we don't have to type request over and
	// over again
	log := log.FromContext(ctx)
	log.V(0).Info("Reconciling")

	// Fetch the Workspace instance
	var ws v1alpha1.Workspace
	if err := r.Get(ctx, req.NamespacedName, &ws); err != nil {
		// we'll ignore not-found errors, since they can't be fixed by an
		// immediate requeue (we'll need to wait for a new notification), and we
		// can get them on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Set garbage collection to use foreground deletion in the event the
	// workspace is deleted
	if !controllerutil.ContainsFinalizer(&ws, metav1.FinalizerDeleteDependents) {
		controllerutil.AddFinalizer(&ws, metav1.FinalizerDeleteDependents)
		if err := r.Update(ctx, &ws); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Prune approval annotations
	annotations, err := r.pruneApprovals(ctx, ws)
	if err != nil {
		return ctrl.Result{}, err
	}
	if !reflect.DeepEqual(ws.Annotations, annotations) {
		ws.Annotations = annotations
		if err := r.Update(ctx, &ws); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Update status one step in the chain at a time. Returns a ready condition.
	ready, backoff := processWorkspaceReconcileStatusChain(ctx, &ws)
	if ready != nil {
		// Add condition to status
		meta.SetStatusCondition(&ws.Status.Conditions, *ready)

		// Ensure phase reflects ready condition
		ws.Status.Phase = setPhase(ready.Reason)

		if err := r.updateStatus(ctx, req, ws.Status); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Non-nil backoff triggers an exponential backoff
	return ctrl.Result{}, backoff
}

// updateStatus actually calls the k8s API to update the workspace resource. To
// avoid errors caused by a stale read cache, it re-retrieves the workspace
// resource and applies a patch.
func (r *WorkspaceReconciler) updateStatus(ctx context.Context, req ctrl.Request, newStatus v1alpha1.WorkspaceStatus) error {
	var ws v1alpha1.Workspace
	if err := r.Get(ctx, req.NamespacedName, &ws); err != nil {
		return err
	}

	ws.Status = newStatus

	return r.Status().Update(ctx, &ws)
}

// processWorkspaceReconcileStatusChain enumerates a list of functions, calling
// them one at time. Each one updates the workspace status and returns a ready
// condition. Depending on the value of the condition, either the list continues
// to be enumerated or the condition is returned. A non-nil error indicates the
// reconcile should be exponentially backed off.
func processWorkspaceReconcileStatusChain(ctx context.Context, ws *v1alpha1.Workspace) (*metav1.Condition, error) {
	var pending *metav1.Condition
	for _, f := range workspaceReconcileStatusChain {
		ready, err := f(ctx, ws)
		if err != nil {
			return nil, err
		}

		if ready == nil {
			continue
		}

		switch ready.Reason {
		case v1alpha1.UnknownReason, v1alpha1.FailureReason:
			// Update condition and trigger exponential back-off
			return ready, errors.New(ready.Message)
		case v1alpha1.DeletionReason:
			// Update condition
			return ready, nil
		case v1alpha1.PendingReason:
			// Last pending wins
			pending = ready
		}
	}

	if pending != nil {
		return pending, nil
	}

	// If no other reason is found, the workspace is ready
	return &metav1.Condition{
		Type:   v1alpha1.WorkspaceReadyCondition,
		Status: metav1.ConditionTrue,
		Reason: v1alpha1.ReadyReason,
	}, nil
}

// setPhase maps the Ready condition's reason field to a phase string
func setPhase(reason string) v1alpha1.WorkspacePhase {
	switch reason {
	case v1alpha1.ReadyReason:
		return v1alpha1.WorkspacePhaseReady
	case v1alpha1.DeletionReason:
		return v1alpha1.WorkspacePhaseDeleting
	case v1alpha1.FailureReason:
		return v1alpha1.WorkspacePhaseError
	case v1alpha1.PendingReason:
		return v1alpha1.WorkspacePhaseInitializing
	default:
		return v1alpha1.WorkspacePhaseUnknown
	}
}

// Determine if workspace is being deleted
func (r *WorkspaceReconciler) handleDeletion(ctx context.Context, ws *v1alpha1.Workspace) (*metav1.Condition, error) {
	if !ws.GetDeletionTimestamp().IsZero() {
		return &metav1.Condition{
			Type:    v1alpha1.WorkspaceReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  v1alpha1.DeletionReason,
			Message: "Workspace is being deleted",
		}, nil
	}
	return nil, nil
}

func (r *WorkspaceReconciler) manageState(ctx context.Context, ws *v1alpha1.Workspace) (*metav1.Condition, error) {
	log := log.FromContext(ctx)

	var secret corev1.Secret
	err := r.Get(ctx, types.NamespacedName{Namespace: ws.Namespace, Name: ws.StateSecretName()}, &secret)
	switch {
	case kerrors.IsNotFound(err):
		if r.BackupProvider != nil {
			return r.restore(ctx, ws)
		}
	case err != nil:
		log.Error(err, "unable to get state secret")
		return nil, err
	default:
		// Make workspace owner of state secret, so that if workspace is deleted
		// so is the state
		if err := controllerutil.SetOwnerReference(ws, &secret, r.Scheme); err != nil {
			log.Error(err, "unable to set state secret ownership")
			return nil, err
		}
		if err := r.Update(ctx, &secret); err != nil {
			return nil, err
		}

		// Retrieve state file secret
		state, err := readState(ctx, &secret)
		if err != nil {
			return nil, err
		}

		// Report state serial number in workspace status
		ws.Status.Serial = &state.Serial

		// Persist outputs from state file to workspace status
		var outputs []*v1alpha1.Output
		for k, v := range state.Outputs {
			outputs = append(outputs, &v1alpha1.Output{Key: k, Value: v.Value})
		}
		if !reflect.DeepEqual(ws.Status.Outputs, outputs) {
			ws.Status.Outputs = outputs
		}

		// Determine if backup should be made
		if r.BackupProvider != nil && !ws.Spec.Ephemeral {
			// Backup if current backup serial doesn't match serial of state
			if ws.Status.BackupSerial == nil || *ws.Status.BackupSerial != state.Serial {
				if err := r.BackupProvider.Backup(ctx, &secret); err != nil {
					return r.sendWarningEvent(err, ws, "BackupError")
				}

				ws.Status.BackupSerial = &state.Serial

				r.recorder.Eventf(ws, "Normal", "BackupSuccessful", "Backed up state #%d", state.Serial)
			}
		}
	}

	return nil, nil
}

func (r *WorkspaceReconciler) addFinalizers(ctx context.Context, ws v1alpha1.Workspace) (v1alpha1.Workspace, error) {
	// Set garbage collection to use foreground deletion in the event the
	// workspace is deleted
	if !controllerutil.ContainsFinalizer(&ws, metav1.FinalizerDeleteDependents) {
		controllerutil.AddFinalizer(&ws, metav1.FinalizerDeleteDependents)
	}
	return ws, nil
}

// Prune invalid approval annotations. Invalid approvals are those that belong
// to runs which are either completed or no longer exist.
func (r *WorkspaceReconciler) pruneApprovals(ctx context.Context, ws v1alpha1.Workspace) (map[string]string, error) {
	if ws.Annotations == nil {
		// Nothing to prune
		return nil, nil
	}

	annotations := makeCopyOfMap(ws.Annotations)

	for k := range annotations {
		if !strings.HasPrefix(k, v1alpha1.ApprovedAnnotationKeyPrefix) {
			// Skip non-approval annotations
			continue
		}

		var run v1alpha1.Run
		objectKey := types.NamespacedName{Namespace: ws.Namespace, Name: v1alpha1.GetRunFromApprovalAnnotationKey(k)}
		err := r.Get(context.TODO(), objectKey, &run)
		if kerrors.IsNotFound(err) {
			// Remove runs that no longer exist
			delete(annotations, k)
			continue
		} else if err != nil {
			return nil, err
		}
		if run.Phase == v1alpha1.RunPhaseCompleted {
			// Remove completed runs
			delete(annotations, k)
		}
	}

	return annotations, nil
}

func (r *WorkspaceReconciler) restore(ctx context.Context, ws *v1alpha1.Workspace) (*metav1.Condition, error) {
	secretKey := types.NamespacedName{Namespace: ws.Namespace, Name: ws.StateSecretName()}
	secret, err := r.BackupProvider.Restore(ctx, secretKey)
	if err != nil {
		return r.sendWarningEvent(err, ws, "RestoreError")
	}
	if secret == nil {
		r.recorder.Eventf(ws, "Normal", "RestoreSkipped", "There is no state to restore")
		return nil, nil
	}

	// Blank out certain fields to avoid errors upon create
	secret.ResourceVersion = ""
	secret.OwnerReferences = nil

	if err := r.Create(ctx, secret); err != nil {
		return r.sendWarningEvent(err, ws, "RestoreError")
	}

	// Parse state file
	state, err := readState(ctx, secret)
	if err != nil {
		return r.sendWarningEvent(err, ws, "RestoreError")
	}

	// Record in status that a backup with the given serial number exists.
	ws.Status.BackupSerial = &state.Serial

	r.recorder.Eventf(ws, "Normal", "RestoreSuccessful", "Restored state #%d", state.Serial)

	return nil, nil
}

// Send warning event as well as propagating error to caller
func (r *WorkspaceReconciler) sendWarningEvent(err error, ws *v1alpha1.Workspace, reason string) (*metav1.Condition, error) {
	r.recorder.Eventf(ws, "Warning", reason, err.Error())
	return nil, err
}

func (r *WorkspaceReconciler) manageBuiltins(ctx context.Context, ws *v1alpha1.Workspace) (*metav1.Condition, error) {
	log := log.FromContext(ctx)

	// Manage ConfigMap containing built-in terraform config for workspace
	var builtins corev1.ConfigMap
	err := r.Get(ctx, types.NamespacedName{Namespace: ws.Namespace, Name: ws.BuiltinsConfigMapName()}, &builtins)
	if kerrors.IsNotFound(err) {
		builtins := *newBuiltinsForWS(ws)

		if err := controllerutil.SetControllerReference(ws, &builtins, r.Scheme); err != nil {
			log.Error(err, "unable to set config map ownership")
			return nil, err
		}

		if err = r.Create(ctx, &builtins); err != nil {
			log.Error(err, "unable to create configmap for builtins")
			return nil, err
		}
		return workspacePending("Creating configmap containing terraform builtins"), nil
	} else if err != nil {
		log.Error(err, "unable to get configmap for builtins")
		return nil, err
	}
	return nil, nil
}

func namespacedNameFromObj(obj controllerutil.Object) types.NamespacedName {
	return types.NamespacedName{Namespace: obj.GetNamespace(), Name: obj.GetName()}
}

func (r *WorkspaceReconciler) manageQueue(ctx context.Context, ws *v1alpha1.Workspace) (*metav1.Condition, error) {
	// Fetch run resources
	runlist := &v1alpha1.RunList{}
	if err := r.List(ctx, runlist, client.InNamespace(ws.Namespace)); err != nil {
		return nil, err
	}

	updateCombinedQueue(ws, runlist.Items)
	return nil, nil
}

// Manage Pod for workspace
func (r *WorkspaceReconciler) managePod(ctx context.Context, ws *v1alpha1.Workspace) (*metav1.Condition, error) {
	log := log.FromContext(ctx)

	var pod corev1.Pod
	err := r.Get(ctx, types.NamespacedName{Namespace: ws.Namespace, Name: ws.PodName()}, &pod)
	if kerrors.IsNotFound(err) {
		pod, err := workspacePod(ws, r.Image)
		if err != nil {
			log.Error(err, "unable to construct pod")
			return nil, err
		}

		if err := controllerutil.SetControllerReference(ws, pod, r.Scheme); err != nil {
			log.Error(err, "unable to set pod ownership")
			return nil, err
		}

		if err = r.Create(ctx, pod); err != nil {
			log.Error(err, "unable to create pod")
			return nil, err
		}

		// TODO: event
		//podOK(ws, v1alpha1.PendingReason, "Creating pod")
		return workspacePending("Creating pod"), nil
	} else if err != nil {
		log.Error(err, "unable to get pod")
		return nil, err
	}

	switch phase := pod.Status.Phase; phase {
	case corev1.PodRunning:
		// TODO: event
		break
	case corev1.PodPending:
		return workspacePending("Pod in pending phase"), nil
	case corev1.PodFailed:
		return workspaceFailure("Pod failed"), nil
	case corev1.PodSucceeded:
		return workspaceFailure("Pod unexpectedly exited"), nil
	default:
		return workspaceUnknown("Pod state unknown"), nil
	}
	return nil, nil
}

// manageRBACForNamespace creates RBAC resources in the Workspace's namespace if
// they don't already exist. They don't belong to the Workspace nor does the
// Workspace rely on them. But a Run in the namespace does; its Pod relies on
// the privileges to carry out k8s API calls (e.g. terraform talking to its
// state residing in a Secret). A user may also add annotations to the
// ServiceAccount to enable things like Workspace Identity.
func (r *WorkspaceReconciler) manageRBACForNamespace(ctx context.Context, ws *v1alpha1.Workspace) (*metav1.Condition, error) {
	log := log.FromContext(ctx)

	var serviceAccount corev1.ServiceAccount
	if err := r.Get(ctx, types.NamespacedName{Namespace: ws.Namespace, Name: ServiceAccountName}, &serviceAccount); err != nil {
		if kerrors.IsNotFound(err) {
			serviceAccount := *newServiceAccountForNamespace(ws)

			if err = r.Create(ctx, &serviceAccount); err != nil {
				log.Error(err, "unable to create service account")
				return nil, err
			}
		} else if err != nil {
			log.Error(err, "unable to get service account")
			return nil, err
		}
	}

	var role rbacv1.Role
	if err := r.Get(ctx, types.NamespacedName{Namespace: ws.Namespace, Name: RoleName}, &role); err != nil {
		if kerrors.IsNotFound(err) {
			role := *newRoleForNamespace(ws)

			if err = r.Create(ctx, &role); err != nil {
				log.Error(err, "unable to create role")
				return nil, err
			}
		} else if err != nil {
			log.Error(err, "unable to get role")
			return nil, err
		}
	}

	var binding rbacv1.RoleBinding
	if err := r.Get(ctx, types.NamespacedName{Namespace: ws.Namespace, Name: RoleBindingName}, &binding); err != nil {
		if kerrors.IsNotFound(err) {
			binding := *newRoleBindingForNamespace(ws)

			if err = r.Create(ctx, &binding); err != nil {
				log.Error(err, "unable to create binding")
				return nil, err
			}
		} else if err != nil {
			log.Error(err, "unable to get binding")
			return nil, err
		}
	}

	return nil, nil
}

func (r *WorkspaceReconciler) managePVC(ctx context.Context, ws *v1alpha1.Workspace) (*metav1.Condition, error) {
	log := log.FromContext(ctx)

	var pvc corev1.PersistentVolumeClaim
	err := r.Get(ctx, types.NamespacedName{Namespace: ws.Namespace, Name: ws.PVCName()}, &pvc)
	if kerrors.IsNotFound(err) {
		pvc := *newPVCForWS(ws)

		if err := controllerutil.SetControllerReference(ws, &pvc, r.Scheme); err != nil {
			log.Error(err, "unable to set PVC ownership")
			return nil, err
		}

		if err = r.Create(ctx, &pvc); err != nil {
			log.Error(err, "unable to create PVC")
			return nil, err
		}
		//cacheOK(ws, v1alpha1.PendingReason, "PVC is being created")
		return workspacePending("Creating PVC"), nil
	} else if err != nil {
		log.Error(err, "unable to get PVC")
		return nil, err
	}

	switch pvc.Status.Phase {
	case corev1.ClaimLost:
		r.recorder.Event(ws, "Warning", "CacheLost", "Cache persistent volume has been lost")
		return nil, errors.New("PVC has lost its persistent volume")
	case corev1.ClaimPending:
		return workspacePending("Cache's PVC in pending state"), nil
	case corev1.ClaimBound:
		return nil, nil
	default:
		return workspaceUnknown("Cache PVC status unknown"), nil
	}
}

func (r *WorkspaceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	blder := ctrl.NewControllerManagedBy(mgr)

	// Watch for changes to primary resource Workspace
	blder = blder.For(&v1alpha1.Workspace{})

	// Watch for changes to secondary resource PVCs and requeue the owner Workspace
	blder = blder.Owns(&corev1.PersistentVolumeClaim{})

	// Watch owned pods
	blder = blder.Owns(&corev1.Pod{})

	// Watch owned config maps (variables)
	blder = blder.Owns(&corev1.ConfigMap{})

	// Watch terraform state files
	blder = blder.Watches(&source.Kind{Type: &corev1.Secret{}}, handler.EnqueueRequestsFromMapFunc(func(o client.Object) []ctrl.Request {
		var isStateFile bool
		for k, v := range o.GetLabels() {
			if k == "tfstate" && v == "true" {
				isStateFile = true
			}
		}
		if !isStateFile {
			return []ctrl.Request{}
		}
		return []ctrl.Request{requestFromObject(o)}
	}))

	// Watch for changes to run resources and requeue the associated Workspace.
	blder = blder.Watches(&source.Kind{Type: &v1alpha1.Run{}}, handler.EnqueueRequestsFromMapFunc(func(o client.Object) []ctrl.Request {
		run := o.(*v1alpha1.Run)
		if run.Workspace != "" {
			return []ctrl.Request{
				{
					NamespacedName: types.NamespacedName{
						Name:      run.Workspace,
						Namespace: o.GetNamespace(),
					},
				},
			}
		}
		return []ctrl.Request{}
	}))

	return blder.Complete(r)
}
