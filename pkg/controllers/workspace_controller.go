package controllers

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"

	"cloud.google.com/go/storage"
	"github.com/leg100/etok/api/etok.dev/v1alpha1"
	"sigs.k8s.io/yaml"

	"sigs.k8s.io/controller-runtime/pkg/log"

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

var (
	// List of functions that update the workspace status
	workspaceReconcileStatusChain []workspaceUpdater
)

type workspaceUpdater func(context.Context, *v1alpha1.Workspace) (*metav1.Condition, error)

type WorkspaceReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	Image         string
	StorageClient *storage.Client
	recorder      record.EventRecorder
}

type WorkspaceReconcilerOption func(r *WorkspaceReconciler)

func WithStorageClient(sc *storage.Client) WorkspaceReconcilerOption {
	return func(r *WorkspaceReconciler) {
		r.StorageClient = sc
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
	workspaceReconcileStatusChain = append(workspaceReconcileStatusChain, r.manageVariables)
	workspaceReconcileStatusChain = append(workspaceReconcileStatusChain, r.manageRBAC)
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
		if ws.Spec.BackupBucket != "" {
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

		if ws.Spec.BackupBucket != "" {
			if ws.Status.BackupSerial == nil || state.Serial != *ws.Status.BackupSerial {
				// Backup the state file and update status
				return r.backup(ctx, ws, &secret, state)
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

func (r *WorkspaceReconciler) backup(ctx context.Context, ws *v1alpha1.Workspace, secret *corev1.Secret, sfile *state) (*metav1.Condition, error) {
	// Re-use client or create if not yet created
	if r.StorageClient == nil {
		var err error
		r.StorageClient, err = storage.NewClient(ctx)
		if err != nil {
			r.recorder.Eventf(ws, "Warning", "BackupError", "Error received when trying to backup state: %w", err)
			return nil, err
		}
	}

	bh := r.StorageClient.Bucket(ws.Spec.BackupBucket)
	_, err := bh.Attrs(ctx)
	if err == storage.ErrBucketNotExist {
		return workspaceFailure(fmt.Sprintf("backup failure: bucket %s not found", ws.Spec.BackupBucket)), nil
	}
	if err != nil {
		r.recorder.Eventf(ws, "Warning", "BackupError", "Error received when trying to backup state: %w", err)
		return nil, err
	}

	oh := bh.Object(ws.BackupObjectName())

	// Marshal state file first to json then to yaml
	y, err := yaml.Marshal(secret)
	if err != nil {
		r.recorder.Eventf(ws, "Warning", "BackupError", "Error received when trying to backup state: %w", err)
		return nil, err
	}

	// Copy state file to GCS
	owriter := oh.NewWriter(ctx)
	_, err = io.Copy(owriter, bytes.NewBuffer(y))
	if err != nil {
		r.recorder.Eventf(ws, "Warning", "BackupError", "Error received when trying to backup state: %w", err)
		return nil, err
	}

	if err := owriter.Close(); err != nil {
		r.recorder.Eventf(ws, "Warning", "BackupError", "Error received when trying to backup state: %w", err)
		return nil, err
	}

	// Update latest backup serial
	ws.Status.BackupSerial = &sfile.Serial

	r.recorder.Eventf(ws, "Normal", "BackupSuccessful", "Backed up state #%d", sfile.Serial)
	return nil, nil
}

func (r *WorkspaceReconciler) restore(ctx context.Context, ws *v1alpha1.Workspace) (*metav1.Condition, error) {
	var secret corev1.Secret

	// Re-use client or create if not yet created
	if r.StorageClient == nil {
		var err error
		r.StorageClient, err = storage.NewClient(ctx)
		if err != nil {
			return nil, err
		}
	}

	bh := r.StorageClient.Bucket(ws.Spec.BackupBucket)
	_, err := bh.Attrs(ctx)
	if err == storage.ErrBucketNotExist {
		return workspaceFailure(fmt.Sprintf("restore failure: bucket %s not found", ws.Spec.BackupBucket)), nil
	} else if err != nil {
		return nil, err
	}

	// Try to retrieve existing backup
	oh := bh.Object(ws.BackupObjectName())
	_, err = oh.Attrs(ctx)
	if err == storage.ErrObjectNotExist {
		r.recorder.Eventf(ws, "Normal", "RestoreSkipped", "There is no state to restore")
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	oreader, err := oh.NewReader(ctx)
	if err != nil {
		r.recorder.Eventf(ws, "Warning", "RestoreError", "Error received when trying to restore state: %w", err)
		return nil, err
	}

	// Copy state file from GCS
	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, oreader)
	if err != nil {
		r.recorder.Eventf(ws, "Warning", "RestoreError", "Error received when trying to restore state: %w", err)
		return nil, err
	}

	// Unmarshal state file into secret obj
	if err := yaml.Unmarshal(buf.Bytes(), &secret); err != nil {
		r.recorder.Eventf(ws, "Warning", "RestoreError", "Error received when trying to restore state: %w", err)
		return nil, err
	}

	if err := oreader.Close(); err != nil {
		r.recorder.Eventf(ws, "Warning", "RestoreError", "Error received when trying to restore state: %w", err)
		return nil, err
	}

	// Blank out certain fields to avoid errors upon create
	secret.ResourceVersion = ""
	secret.OwnerReferences = nil

	if err := r.Create(ctx, &secret); err != nil {
		r.recorder.Eventf(ws, "Warning", "RestoreError", "Error received when trying to restore state: %w", err)
		return nil, err
	}

	// Parse state file
	state, err := readState(ctx, &secret)
	if err != nil {
		r.recorder.Eventf(ws, "Warning", "RestoreError", "Error received when trying to restore state: %w", err)
		return nil, err
	}

	// Record in status that a backup with the given serial number exists.
	ws.Status.BackupSerial = &state.Serial

	r.recorder.Eventf(ws, "Normal", "RestoreSuccessful", "Restored state #%d", state.Serial)

	return nil, nil
}

func (r *WorkspaceReconciler) manageVariables(ctx context.Context, ws *v1alpha1.Workspace) (*metav1.Condition, error) {
	log := log.FromContext(ctx)

	// Manage ConfigMap containing variables for workspace
	var variables corev1.ConfigMap
	err := r.Get(ctx, types.NamespacedName{Namespace: ws.Namespace, Name: ws.VariablesConfigMapName()}, &variables)
	if kerrors.IsNotFound(err) {
		variables := *newVariablesForWS(ws)

		if err := controllerutil.SetControllerReference(ws, &variables, r.Scheme); err != nil {
			log.Error(err, "unable to set config map ownership")
			return nil, err
		}

		if err = r.Create(ctx, &variables); err != nil {
			log.Error(err, "unable to create configmap for variables")
			return nil, err
		}
		return workspacePending("Creating configmap containing terraform variables"), nil
	} else if err != nil {
		log.Error(err, "unable to get configmap for variables")
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

func (r *WorkspaceReconciler) manageRBAC(ctx context.Context, ws *v1alpha1.Workspace) (*metav1.Condition, error) {
	log := log.FromContext(ctx)

	// Manage RBAC role for workspace
	var role rbacv1.Role
	if err := r.Get(ctx, namespacedNameFromObj(ws), &role); err != nil {
		if kerrors.IsNotFound(err) {
			role := *newRoleForWS(ws)

			if err := controllerutil.SetControllerReference(ws, &role, r.Scheme); err != nil {
				log.Error(err, "unable to set role ownership")
				return nil, err
			}

			if err = r.Create(ctx, &role); err != nil {
				log.Error(err, "unable to create role")
				return nil, err
			}
			return workspacePending("Creating RBAC role"), nil
		} else if err != nil {
			log.Error(err, "unable to get role")
			return nil, err
		}
	}

	// Manage RBAC role binding for workspace
	var binding rbacv1.RoleBinding
	if err := r.Get(ctx, namespacedNameFromObj(ws), &binding); err != nil {
		if kerrors.IsNotFound(err) {
			binding := *newRoleBindingForWS(ws)

			if err := controllerutil.SetControllerReference(ws, &binding, r.Scheme); err != nil {
				log.Error(err, "unable to set binding ownership")
				return nil, err
			}

			if err = r.Create(ctx, &binding); err != nil {
				log.Error(err, "unable to create binding")
				return nil, err
			}
			return workspacePending("Creating RBAC binding"), nil
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
