package controllers

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
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

type workspaceUpdater func(context.Context, *v1alpha1.Workspace) error

type WorkspaceReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	Image         string
	StorageClient *storage.Client
}

type WorkspaceReconcilerOption func(r *WorkspaceReconciler)

func WithStorageClient(sc *storage.Client) WorkspaceReconcilerOption {
	return func(r *WorkspaceReconciler) {
		r.StorageClient = sc
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
	workspaceReconcileStatusChain = append(workspaceReconcileStatusChain, r.manageVariables)
	workspaceReconcileStatusChain = append(workspaceReconcileStatusChain, r.manageRBAC)
	workspaceReconcileStatusChain = append(workspaceReconcileStatusChain, r.manageState)
	workspaceReconcileStatusChain = append(workspaceReconcileStatusChain, r.managePVC)
	workspaceReconcileStatusChain = append(workspaceReconcileStatusChain, r.managePod)
	workspaceReconcileStatusChain = append(workspaceReconcileStatusChain, r.manageQueue)
	workspaceReconcileStatusChain = append(workspaceReconcileStatusChain, managePhase)

	return r
}

// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=roles,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=rolebindings,verbs=get;list;watch;create;update;patch;delete

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

	// Determine if workspace is being deleted
	if !ws.GetDeletionTimestamp().IsZero() {
		// It's being deleted...set phase if not done already
		if ws.Status.Phase != v1alpha1.WorkspacePhaseDeleting {
			ws.Status.Phase = v1alpha1.WorkspacePhaseDeleting
			if err := r.Status().Update(ctx, &ws); err != nil {
				return ctrl.Result{}, err
			}
		}

		// Cease reconciliation
		return ctrl.Result{}, nil
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

	// Update status struct
	reconcileErr := processWorkspaceReconcileStatusChain(ctx, &ws)

	if err := r.updateStatus(ctx, req, ws.Status); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, reconcileErr
}

func (r *WorkspaceReconciler) updateStatus(ctx context.Context, req ctrl.Request, newStatus v1alpha1.WorkspaceStatus) error {
	var ws v1alpha1.Workspace
	if err := r.Get(ctx, req.NamespacedName, &ws); err != nil {
		return err
	}

	patch := client.MergeFrom(ws.DeepCopy())
	ws.Status = newStatus

	return r.Status().Patch(ctx, &ws, patch)
}

// Update status. Bail out early if an error is returned.
func processWorkspaceReconcileStatusChain(ctx context.Context, ws *v1alpha1.Workspace) error {
	for _, f := range workspaceReconcileStatusChain {
		if err := f(ctx, ws); err != nil {
			return err
		}
	}
	return nil
}

func (r *WorkspaceReconciler) manageState(ctx context.Context, ws *v1alpha1.Workspace) error {
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
		return err
	default:
		// Make workspace owner of state secret, so that if workspace is deleted
		// so is the state
		if err := controllerutil.SetOwnerReference(ws, &secret, r.Scheme); err != nil {
			log.Error(err, "unable to set state secret ownership")
			return err
		}
		if err := r.Update(ctx, &secret); err != nil {
			return err
		}

		// Retrieve state file secret
		state, err := r.readState(ctx, ws, &secret)
		if err != nil {
			return err
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

	return nil
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

	// Make a copy of annotations, leaving out approvals
	var hasApproval bool
	annotations := make(map[string]string)
	for k, v := range ws.Annotations {
		if strings.HasPrefix(k, v1alpha1.ApprovedAnnotationKeyPrefix) {
			hasApproval = true
			continue
		}
		annotations[k] = v
	}

	if !hasApproval {
		// No approvals found, so there's nothing to re-add
		return nil, nil
	}

	// Enumerate runs, and re-add approvals accordingly
	runlist := &v1alpha1.RunList{}
	if err := r.List(context.TODO(), runlist, client.InNamespace(ws.Namespace)); err != nil {
		return nil, err
	}
	for _, run := range runlist.Items {
		if run.Phase == v1alpha1.RunPhaseCompleted {
			// Don't re-add completed runs
			continue
		}

		if metav1.HasAnnotation(ws.ObjectMeta, run.ApprovedAnnotationKey()) {
			// Re-add approval
			annotations[run.ApprovedAnnotationKey()] = "approved"
		}
	}

	return annotations, nil
}

func (r *WorkspaceReconciler) readState(ctx context.Context, ws *v1alpha1.Workspace, secret *corev1.Secret) (*state, error) {
	data, ok := secret.Data["tfstate"]
	if !ok {
		return nil, errors.New("Expected key tfstate not found in state secret")
	}

	// Return a gzip reader that decompresses on the fly
	gr, err := gzip.NewReader(bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}

	// Unmarshal state file
	var s state
	if err := json.NewDecoder(gr).Decode(&s); err != nil {
		return nil, err
	}

	return &s, nil
}

func (r *WorkspaceReconciler) backup(ctx context.Context, ws *v1alpha1.Workspace, secret *corev1.Secret, sfile *state) error {
	// Re-use client or create if not yet created
	if r.StorageClient == nil {
		var err error
		r.StorageClient, err = storage.NewClient(ctx)
		if err != nil {
			backupFailure(ws, v1alpha1.ClientCreateReason, err.Error())
			return err
		}
	}

	bh := r.StorageClient.Bucket(ws.Spec.BackupBucket)
	_, err := bh.Attrs(ctx)
	if err == storage.ErrBucketNotExist {
		backupFailure(ws, v1alpha1.BucketNotFoundReason, err.Error())
		return err
	} else if err != nil {
		backupFailure(ws, v1alpha1.UnexpectedErrorReason, err.Error())
		return err
	}

	oh := bh.Object(ws.BackupObjectName())

	// Marshal state file first to json then to yaml
	y, err := yaml.Marshal(secret)
	if err != nil {
		backupFailure(ws, v1alpha1.UnexpectedErrorReason, err.Error())
		return err
	}

	// Copy state file to GCS
	owriter := oh.NewWriter(ctx)
	_, err = io.Copy(owriter, bytes.NewBuffer(y))
	if err != nil {
		backupFailure(ws, v1alpha1.UnexpectedErrorReason, err.Error())
		return err
	}

	if err := owriter.Close(); err != nil {
		backupFailure(ws, v1alpha1.UnexpectedErrorReason, err.Error())
		return err
	}

	// Update latest backup serial
	ws.Status.BackupSerial = &sfile.Serial

	backupOK(ws, v1alpha1.BackupSuccessfulReason, "State successfully backed up")
	return nil
}

func (r *WorkspaceReconciler) restore(ctx context.Context, ws *v1alpha1.Workspace) error {
	var secret corev1.Secret

	// Re-use client or create if not yet created
	if r.StorageClient == nil {
		var err error
		r.StorageClient, err = storage.NewClient(ctx)
		if err != nil {
			restoreFailure(ws, v1alpha1.ClientCreateReason, err.Error())
			return err
		}
	}

	bh := r.StorageClient.Bucket(ws.Spec.BackupBucket)
	_, err := bh.Attrs(ctx)
	if err == storage.ErrBucketNotExist {
		restoreFailure(ws, v1alpha1.BucketNotFoundReason, err.Error())
		return err
	} else if err != nil {
		restoreFailure(ws, v1alpha1.UnexpectedErrorReason, err.Error())
		return err
	}

	// Try to retrieve existing backup
	oh := bh.Object(ws.BackupObjectName())
	_, err = oh.Attrs(ctx)
	if err == storage.ErrObjectNotExist {
		restoreOK(ws, v1alpha1.NothingToRestoreReason, "No backup was found to restore")
		return nil
	} else if err != nil {
		restoreFailure(ws, v1alpha1.UnexpectedErrorReason, err.Error())
		return err
	}

	oreader, err := oh.NewReader(ctx)
	if err != nil {
		restoreFailure(ws, v1alpha1.UnexpectedErrorReason, err.Error())
		return err
	}

	// Copy state file from GCS
	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, oreader)
	if err != nil {
		restoreFailure(ws, v1alpha1.UnexpectedErrorReason, err.Error())
		return err
	}

	// Unmarshal state file into secret obj
	if err := yaml.Unmarshal(buf.Bytes(), &secret); err != nil {
		restoreFailure(ws, v1alpha1.UnexpectedErrorReason, err.Error())
		return err
	}

	if err := oreader.Close(); err != nil {
		restoreFailure(ws, v1alpha1.UnexpectedErrorReason, err.Error())
		return err
	}

	// Blank out resource version to avoid error upon create
	secret.ResourceVersion = ""

	if err := r.Create(ctx, &secret); err != nil {
		restoreFailure(ws, v1alpha1.UnexpectedErrorReason, err.Error())
		return err
	}

	restoreOK(ws, v1alpha1.RestoreSuccessfulReason, "State successfully restored")
	return nil
}

// Set phase, which is an aggregation or summarisation of the workspace's error
// conditions. In order of precedence: If there is at least one true error
// condition then the phase will be set to error. If there is at least one
// unknown condition then the phase will be set to unknown. If there is at least
// one true condition with a pending reason then the phase will be set to
// pending.  If all conditions are false only then is the phase set to ready.
func managePhase(ctx context.Context, ws *v1alpha1.Workspace) error {
	var phase = v1alpha1.WorkspacePhaseReady

	for _, cond := range ws.Status.Conditions {
		switch cond.Status {
		case metav1.ConditionTrue:
			ws.Status.Phase = v1alpha1.WorkspacePhaseError
			return nil
		case metav1.ConditionFalse:
			if cond.Reason == v1alpha1.PendingReason && phase != v1alpha1.WorkspacePhaseUnknown {
				phase = v1alpha1.WorkspacePhaseInitializing
			}
		case metav1.ConditionUnknown:
			phase = v1alpha1.WorkspacePhaseUnknown
		}
	}

	ws.Status.Phase = phase
	return nil
}

func (r *WorkspaceReconciler) manageVariables(ctx context.Context, ws *v1alpha1.Workspace) error {
	log := log.FromContext(ctx)

	// Manage ConfigMap containing variables for workspace
	var variables corev1.ConfigMap
	err := r.Get(ctx, types.NamespacedName{Namespace: ws.Namespace, Name: ws.VariablesConfigMapName()}, &variables)
	if kerrors.IsNotFound(err) {
		variables := *newVariablesForWS(ws)

		if err := controllerutil.SetControllerReference(ws, &variables, r.Scheme); err != nil {
			log.Error(err, "unable to set config map ownership")
			return err
		}

		if err = r.Create(ctx, &variables); err != nil {
			log.Error(err, "unable to create configmap for variables")
			return err
		}
	} else if err != nil {
		log.Error(err, "unable to get configmap for variables")
		return err
	}
	return nil
}

func namespacedNameFromObj(obj controllerutil.Object) types.NamespacedName {
	return types.NamespacedName{Namespace: obj.GetNamespace(), Name: obj.GetName()}
}

func (r *WorkspaceReconciler) manageQueue(ctx context.Context, ws *v1alpha1.Workspace) error {
	// Fetch run resources
	runlist := &v1alpha1.RunList{}
	if err := r.List(ctx, runlist, client.InNamespace(ws.Namespace)); err != nil {
		return err
	}

	updateCombinedQueue(ws, runlist.Items)
	return nil
}

// Manage Pod for workspace
func (r *WorkspaceReconciler) managePod(ctx context.Context, ws *v1alpha1.Workspace) error {
	log := log.FromContext(ctx)

	var pod corev1.Pod
	err := r.Get(ctx, types.NamespacedName{Namespace: ws.Namespace, Name: ws.PodName()}, &pod)
	if kerrors.IsNotFound(err) {
		pod, err := workspacePod(ws, r.Image)
		if err != nil {
			log.Error(err, "unable to construct pod")
			return err
		}

		if err := controllerutil.SetControllerReference(ws, pod, r.Scheme); err != nil {
			log.Error(err, "unable to set pod ownership")
			return err
		}

		if err = r.Create(ctx, pod); err != nil {
			log.Error(err, "unable to create pod")
			return err
		}

		podOK(ws, v1alpha1.PendingReason, "Creating pod")
		return nil

	} else if err != nil {
		log.Error(err, "unable to get pod")
		return err
	}

	switch phase := pod.Status.Phase; phase {
	case corev1.PodRunning:
		podOK(ws, string(phase), "")
	case corev1.PodPending:
		podOK(ws, v1alpha1.PendingReason, "Pod in pending phase")
	case corev1.PodFailed:
		podFailure(ws, string(phase), "Pod unexpectedly failed")
	case corev1.PodSucceeded:
		podFailure(ws, string(phase), "Pod unexpectedly completed")
	default:
		podUnknown(ws, string(phase), "")
	}
	return nil
}

func (r *WorkspaceReconciler) manageRBAC(ctx context.Context, ws *v1alpha1.Workspace) error {
	log := log.FromContext(ctx)

	// Manage RBAC role for workspace
	var role rbacv1.Role
	if err := r.Get(ctx, namespacedNameFromObj(ws), &role); err != nil {
		if kerrors.IsNotFound(err) {
			role := *newRoleForWS(ws)

			if err := controllerutil.SetControllerReference(ws, &role, r.Scheme); err != nil {
				log.Error(err, "unable to set role ownership")
				return err
			}

			if err = r.Create(ctx, &role); err != nil {
				log.Error(err, "unable to create role")
				return err
			}
		} else if err != nil {
			log.Error(err, "unable to get role")
			return err
		}
	}

	// Manage RBAC role binding for workspace
	var binding rbacv1.RoleBinding
	if err := r.Get(ctx, namespacedNameFromObj(ws), &binding); err != nil {
		if kerrors.IsNotFound(err) {
			binding := *newRoleBindingForWS(ws)

			if err := controllerutil.SetControllerReference(ws, &binding, r.Scheme); err != nil {
				log.Error(err, "unable to set binding ownership")
				return err
			}

			if err = r.Create(ctx, &binding); err != nil {
				log.Error(err, "unable to create binding")
				return err
			}
		} else if err != nil {
			log.Error(err, "unable to get binding")
			return err
		}
	}

	return nil
}

func (r *WorkspaceReconciler) managePVC(ctx context.Context, ws *v1alpha1.Workspace) error {
	log := log.FromContext(ctx)

	var pvc corev1.PersistentVolumeClaim
	err := r.Get(ctx, types.NamespacedName{Namespace: ws.Namespace, Name: ws.PVCName()}, &pvc)
	if kerrors.IsNotFound(err) {
		pvc := *newPVCForWS(ws)

		if err := controllerutil.SetControllerReference(ws, &pvc, r.Scheme); err != nil {
			log.Error(err, "unable to set PVC ownership")
			return err
		}

		if err = r.Create(ctx, &pvc); err != nil {
			log.Error(err, "unable to create PVC")
			return err
		}
		cacheOK(ws, v1alpha1.PendingReason, "PVC is being created")
		return nil
	} else if err != nil {
		log.Error(err, "unable to get PVC")
		return err
	}

	switch pvc.Status.Phase {
	case corev1.ClaimBound:
		cacheOK(ws, v1alpha1.CacheBoundReason, "Cache's PVC successfully bound to PV")
	case corev1.ClaimLost:
		cacheFailure(ws, v1alpha1.CacheLostReason, "Persistent volume does not exist any longer")
	case corev1.ClaimPending:
		cacheOK(ws, v1alpha1.PendingReason, "Cache's PVC in pending state")
	}

	return nil
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
	blder = blder.Watches(&source.Kind{Type: &corev1.ConfigMap{}}, handler.EnqueueRequestsFromMapFunc(func(o client.Object) []ctrl.Request {
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
