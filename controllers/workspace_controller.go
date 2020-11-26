package controllers

import (
	"context"
	"fmt"
	"reflect"
	"regexp"
	"strings"

	"github.com/go-logr/logr"
	"github.com/leg100/stok/api/stok.goalspike.com/v1alpha1"
	"github.com/leg100/stok/pkg/labels"
	"github.com/leg100/stok/pkg/runner"
	"github.com/leg100/stok/scheme"
	"github.com/leg100/stok/util/slice"
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
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var approvalAnnotationKeyRegex = regexp.MustCompile("approvals.stok.goalspike.com/[-a-z0-9]+")

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

// +kubebuilder:rbac:groups=stok.goalspike.com,resources=workspaces,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=stok.goalspike.com,resources=workspaces/status,verbs=get;update;patch

// Reconcile reads that state of the cluster for a Workspace object and makes changes based on the state read
// and what is in the Workspace.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *WorkspaceReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	reqLogger := r.Log.WithValues("workspace", req.NamespacedName)
	reqLogger.V(0).Info("Reconciling Workspace")

	// Fetch the Workspace instance
	instance := &v1alpha1.Workspace{}
	err := r.Get(context.TODO(), req.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		reqLogger.Error(err, "Error retrieving workspace")
		return ctrl.Result{}, err
	}

	// Because it is a required attribute we need to set the queue status to an empty array if it
	// is not already set
	// TODO: need to update ws after setting these defaults
	if instance.Status.Queue == nil {
		instance.Status.Queue = []string{}
	}

	if instance.GetDeletionTimestamp().IsZero() {
		// Workspace not being deleted
		if !slice.ContainsString(instance.GetFinalizers(), metav1.FinalizerDeleteDependents) {
			// Instruct garbage collector to only delete workspace once its dependents are deleted
			instance.SetFinalizers([]string{metav1.FinalizerDeleteDependents})
			if err := r.Update(context.TODO(), instance); err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
		// Workspace is being deleted
		instance.Status.Phase = v1alpha1.WorkspacePhaseDeleting
		if err := r.Status().Update(context.TODO(), instance); err != nil {
			return ctrl.Result{}, err
		}

		// Cease reconciliation
		return ctrl.Result{}, nil
	}

	// Manage ConfigMap for workspace
	configMap := newConfigMapForCR(instance)
	if err := r.manageControllee(instance, reqLogger, configMap); err != nil {
		return ctrl.Result{}, err
	}

	// Manage PVC for workspace cache dir
	pvc := newPVCForCR(instance)
	if err := r.manageControllee(instance, reqLogger, pvc); err != nil {
		return ctrl.Result{}, err
	}

	// Manage Pod for workspace
	pod := r.newPodForCR(instance)
	if err := r.manageControllee(instance, reqLogger, pod); err != nil {
		return ctrl.Result{}, err
	}

	// Set workspace phase status
	if err := r.setPhase(instance, pod); err != nil {
		return ctrl.Result{}, fmt.Errorf("Unable to set workspace phase: %w", err)
	}

	// Update run queue
	if err := updateQueue(r.Client, instance); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to update queue: %w", err)
	}

	// Garbage collect approval annotations
	if annotations := instance.Annotations; annotations != nil {
		updatedAnnotations := make(map[string]string)
		for k, v := range annotations {
			if approvalAnnotationKeyRegex.MatchString(k) {
				run := strings.Split(k, "/")[1]
				if slice.ContainsString(instance.Status.Queue, run) {
					// Run is still enqueued so keep its annotation
					updatedAnnotations[k] = v
				}
			}
		}
		if !reflect.DeepEqual(annotations, updatedAnnotations) {
			instance.Annotations = updatedAnnotations
			if err := r.Update(context.TODO(), instance); err != nil {
				return ctrl.Result{}, fmt.Errorf("Failed to update approval annotations: %w", err)
			}
		}
	}

	return ctrl.Result{}, nil
}

func (r *WorkspaceReconciler) setPhase(ws *v1alpha1.Workspace, pod *corev1.Pod) error {
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
	return r.Status().Update(context.TODO(), ws)
}

// For a given go object, return the corresponding Kind. A wrapper for scheme.ObjectKinds, which
// returns all possible GVKs for a go object, but the wrapper returns only the Kind, checking only
// that at least one GVK exists. (The Kind should be the same for all GVKs).
// TODO: could just use reflect.TypeOf instead...
func getKindFromObject(scheme *runtime.Scheme, obj runtime.Object) (string, error) {
	gvks, _, err := scheme.ObjectKinds(obj)
	if err != nil {
		return "", err
	}
	if len(gvks) == 0 {
		return "", fmt.Errorf("no kind found for obj %v", obj)
	}
	return gvks[0].Kind, nil
}

func (r *WorkspaceReconciler) manageControllee(ws *v1alpha1.Workspace, logger logr.Logger, controllee controllerutil.Object) error {
	kind, err := getKindFromObject(r.Scheme, controllee)
	if err != nil {
		return err
	}

	log := logger.WithValues("Controllee.Kind", kind)

	// Set Workspace instance as the owner and controller
	if err := controllerutil.SetControllerReference(ws, controllee, r.Scheme); err != nil {
		log.Error(err, "Unable to set controller reference")
		return err
	}

	controlleeKey, err := client.ObjectKeyFromObject(controllee)
	if err != nil {
		return err
	}

	err = r.Get(context.TODO(), controlleeKey, controllee)
	if errors.IsNotFound(err) {
		if err = r.Create(context.TODO(), controllee); err != nil {
			log.Error(err, "Failed to create controllee", "Controllee.Name", controllee.GetName())
			return err
		}
		log.Info("Created controllee", "Controllee.Name", controllee.GetName())
	} else if err != nil {
		log.Error(err, "Error retrieving controllee")
		return err
	}

	return nil
}

func newConfigMapForCR(cr *v1alpha1.Workspace) *corev1.ConfigMap {
	configMap := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      v1alpha1.BackendConfigMapName(cr.GetName()),
			Namespace: cr.Namespace,
		},
		Data: map[string]string{
			v1alpha1.BackendTypeFilename:   v1alpha1.BackendEmptyConfig(cr.Spec.Backend.Type),
			v1alpha1.BackendConfigFilename: v1alpha1.BackendConfig(cr.Spec.Backend.Config),
		},
	}

	// Set stok's common labels
	labels.SetCommonLabels(configMap)
	// Permit filtering stok resources by component
	labels.SetLabel(configMap, labels.WorkspaceComponent)

	return configMap
}

func newPVCForCR(cr *v1alpha1.Workspace) controllerutil.Object {
	size := v1alpha1.WorkspaceDefaultCacheSize
	if cr.Spec.Cache.Size != "" {
		size = cr.Spec.Cache.Size
	}

	pvc := &corev1.PersistentVolumeClaim{
		TypeMeta: metav1.TypeMeta{
			Kind:       "PersistentVolumeClaim",
			APIVersion: "",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name,
			Namespace: cr.Namespace,
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

	// Set stok's common labels
	labels.SetCommonLabels(pvc)
	// Permit filtering stok resources by component
	labels.SetLabel(pvc, labels.WorkspaceComponent)

	if cr.Spec.Cache.StorageClass != "" {
		pvc.Spec.StorageClassName = &cr.Spec.Cache.StorageClass
	}

	return pvc
}

func (r *WorkspaceReconciler) newPodForCR(cr *v1alpha1.Workspace) *corev1.Pod {
	return runner.WorkspacePod(cr, r.Image)
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
		ToRequests: handler.ToRequestsFunc(func(a handler.MapObject) []reconcile.Request {
			run := a.Object.(*v1alpha1.Run)
			if run.GetWorkspace() != "" {
				return []reconcile.Request{
					{
						NamespacedName: types.NamespacedName{
							Name:      run.GetWorkspace(),
							Namespace: a.Meta.GetNamespace(),
						},
					},
				}
			}
			return []reconcile.Request{}
		}),
	})

	return blder.Complete(r)
}
