package controllers

import (
	"context"
	"reflect"
	"regexp"
	"strings"

	"github.com/go-logr/logr"
	"github.com/leg100/etok/api/etok.dev/v1alpha1"
	"github.com/leg100/etok/pkg/labels"
	"github.com/leg100/etok/pkg/runner"
	"github.com/leg100/etok/pkg/scheme"
	"github.com/leg100/etok/pkg/util/slice"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var approvalAnnotationKeyRegex = regexp.MustCompile("approvals.etok.dev/[-a-z0-9]+")

type WorkspaceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger
	Image  string
}

func NewWorkspaceReconciler(cl client.Client, image string) *WorkspaceReconciler {
	return &WorkspaceReconciler{
		Client: cl,
		Scheme: scheme.Scheme,
		Log:    ctrl.Log.WithName("controllers").WithName("Workspace"),
		Image:  image,
	}
}

// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete

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
func (r *WorkspaceReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("workspace", req.NamespacedName)
	log.V(0).Info("Reconciling")

	// Fetch the Workspace instance
	var ws v1alpha1.Workspace
	if err := r.Get(ctx, req.NamespacedName, &ws); err != nil {
		log.Error(err, "unable to get workspace")
		// we'll ignore not-found errors, since they can't be fixed by an
		// immediate requeue (we'll need to wait for a new notification), and we
		// can get them on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if ws.GetDeletionTimestamp().IsZero() {
		// Workspace not being deleted
		if !slice.ContainsString(ws.GetFinalizers(), metav1.FinalizerDeleteDependents) {
			// Instruct garbage collector to only delete workspace once its dependents are deleted
			ws.SetFinalizers([]string{metav1.FinalizerDeleteDependents})
			if err := r.Update(ctx, &ws); err != nil {
				log.Error(err, "unable to set finalizer")
				return ctrl.Result{}, err
			}
		}
	} else {
		// Workspace is being deleted
		ws.Status.Phase = v1alpha1.WorkspacePhaseDeleting
		if err := r.Status().Update(ctx, &ws); err != nil {
			log.Error(err, "unable to set phase")
			return ctrl.Result{}, err
		}

		// Cease reconciliation
		return ctrl.Result{}, nil
	}

	// Manage ConfigMap for workspace
	var configMap corev1.ConfigMap
	if err := r.Get(ctx, types.NamespacedName{Namespace: req.Namespace, Name: v1alpha1.BackendConfigMapName(ws.Name)}, &configMap); err != nil {
		if errors.IsNotFound(err) {
			configMap := *newConfigMapForWS(&ws)

			if err := controllerutil.SetControllerReference(&ws, &configMap, r.Scheme); err != nil {
				log.Error(err, "unable to set config map ownership")
				return ctrl.Result{}, err
			}

			if err = r.Create(ctx, &configMap); err != nil {
				log.Error(err, "unable to create config map")
				return ctrl.Result{}, err
			}
		} else if err != nil {
			log.Error(err, "unable to get config map")
			return ctrl.Result{}, err
		}
	}

	// Manage PVC for workspace
	var pvc corev1.PersistentVolumeClaim
	if err := r.Get(ctx, types.NamespacedName{Namespace: req.Namespace, Name: ws.PVCName()}, &pvc); err != nil {
		if errors.IsNotFound(err) {
			pvc := *newPVCForWS(&ws)

			if err := controllerutil.SetControllerReference(&ws, &pvc, r.Scheme); err != nil {
				log.Error(err, "unable to set PVC ownership")
				return ctrl.Result{}, err
			}

			if err = r.Create(ctx, &pvc); err != nil {
				log.Error(err, "unable to create PVC")
				return ctrl.Result{}, err
			}
		} else if err != nil {
			log.Error(err, "unable to get PVC")
			return ctrl.Result{}, err
		}
	}

	// Manage Pod for workspace
	var pod corev1.Pod
	if err := r.Get(ctx, types.NamespacedName{Namespace: req.Namespace, Name: ws.PodName()}, &pod); err != nil {
		if errors.IsNotFound(err) {
			pod := *runner.NewWorkspacePod(&ws, r.Image)

			if err := controllerutil.SetControllerReference(&ws, &pod, r.Scheme); err != nil {
				log.Error(err, "unable to set pod ownership")
				return ctrl.Result{}, err
			}

			if err = r.Create(ctx, &pod); err != nil {
				log.Error(err, "unable to create pod")
				return ctrl.Result{}, err
			}
		} else if err != nil {
			log.Error(err, "unable to get pod")
			return ctrl.Result{}, err
		}
	}

	// Set workspace phase status
	if err := r.setPhase(ctx, &ws, &pod); err != nil {
		log.Error(err, "unable to set phase")
		return ctrl.Result{}, err
	}

	// Update run queue
	if err := updateQueue(r.Client, &ws); err != nil {
		log.Error(err, "unable to update queue")
		return ctrl.Result{}, err
	}

	// Garbage collect approval annotations
	if annotations := ws.Annotations; annotations != nil {
		updatedAnnotations := make(map[string]string)
		for k, v := range annotations {
			if approvalAnnotationKeyRegex.MatchString(k) {
				run := strings.Split(k, "/")[1]
				if slice.ContainsString(ws.Status.Queue, run) {
					// Run is still enqueued so keep its annotation
					updatedAnnotations[k] = v
				}
			}
		}
		if !reflect.DeepEqual(annotations, updatedAnnotations) {
			ws.Annotations = updatedAnnotations
			if err := r.Update(ctx, &ws); err != nil {
				log.Error(err, "failed to update approval annotations")
				return ctrl.Result{}, err
			}
		}
	}

	return ctrl.Result{}, nil
}

func (r *WorkspaceReconciler) setPhase(ctx context.Context, ws *v1alpha1.Workspace, pod *corev1.Pod) error {
	switch pod.Status.Phase {
	case corev1.PodPending:
		ws.Status.Phase = v1alpha1.WorkspacePhaseInitializing
	case corev1.PodRunning:
		ws.Status.Phase = v1alpha1.WorkspacePhaseReady
	case corev1.PodSucceeded, corev1.PodFailed:
		ws.Status.Phase = v1alpha1.WorkspacePhaseError
	default:
		ws.Status.Phase = v1alpha1.WorkspacePhaseUnknown
	}
	return r.Status().Update(ctx, ws)
}

func newConfigMapForWS(ws *v1alpha1.Workspace) *corev1.ConfigMap {
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      v1alpha1.BackendConfigMapName(ws.GetName()),
			Namespace: ws.Namespace,
		},
		Data: map[string]string{
			v1alpha1.BackendTypeFilename:   v1alpha1.BackendEmptyConfig(ws.Spec.Backend.Type),
			v1alpha1.BackendConfigFilename: v1alpha1.BackendConfig(ws.Spec.Backend.Config),
		},
	}

	// Set etok's common labels
	labels.SetCommonLabels(configMap)
	// Permit filtering etok resources by component
	labels.SetLabel(configMap, labels.WorkspaceComponent)

	return configMap
}

func newPVCForWS(ws *v1alpha1.Workspace) *corev1.PersistentVolumeClaim {
	size := v1alpha1.WorkspaceDefaultCacheSize
	if ws.Spec.Cache.Size != "" {
		size = ws.Spec.Cache.Size
	}

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ws.Name,
			Namespace: ws.Namespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse(size),
				},
			},
		},
	}

	// Set etok's common labels
	labels.SetCommonLabels(pvc)
	// Permit filtering etok resources by component
	labels.SetLabel(pvc, labels.WorkspaceComponent)

	if ws.Spec.Cache.StorageClass != "" {
		pvc.Spec.StorageClassName = &ws.Spec.Cache.StorageClass
	}

	return pvc
}

func (r *WorkspaceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	blder := ctrl.NewControllerManagedBy(mgr)

	// Watch for changes to primary resource Workspace
	blder = blder.For(&v1alpha1.Workspace{})

	// Watch for changes to secondary resource PVCs and requeue the owner Workspace
	blder = blder.Owns(&corev1.PersistentVolumeClaim{})

	// Watch owned configmaps
	blder = blder.Owns(&corev1.ConfigMap{})

	// Watch owned pods
	blder = blder.Owns(&corev1.Pod{})

	// Watch for changes to run resources and requeue the associated Workspace.
	blder = blder.Watches(&source.Kind{Type: &v1alpha1.Run{}}, &handler.EnqueueRequestsFromMapFunc{
		ToRequests: handler.ToRequestsFunc(func(a handler.MapObject) []ctrl.Request {
			run := a.Object.(*v1alpha1.Run)
			if run.Workspace != "" {
				return []ctrl.Request{
					{
						NamespacedName: types.NamespacedName{
							Name:      run.Workspace,
							Namespace: a.Meta.GetNamespace(),
						},
					},
				}
			}
			return []ctrl.Request{}
		}),
	})

	return blder.Complete(r)
}
