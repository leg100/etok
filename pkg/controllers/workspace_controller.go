package controllers

import (
	"context"
	"reflect"
	"regexp"
	"strings"

	"github.com/leg100/etok/api/etok.dev/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/leg100/etok/pkg/labels"
	"github.com/leg100/etok/pkg/scheme"
	"github.com/leg100/etok/pkg/util/slice"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
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
	Image  string
}

func NewWorkspaceReconciler(cl client.Client, image string) *WorkspaceReconciler {
	return &WorkspaceReconciler{
		Client: cl,
		Scheme: scheme.Scheme,
		Image:  image,
	}
}

// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=roles,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=rolebindings,verbs=get;list;watch;create;update;patch;delete
//
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

	if ws.GetDeletionTimestamp().IsZero() {
		// Workspace not being deleted
		if !slice.ContainsString(ws.GetFinalizers(), metav1.FinalizerDeleteDependents) {
			// Instruct garbage collector to only delete workspace once its dependents are deleted
			ws.SetFinalizers([]string{metav1.FinalizerDeleteDependents})
			if err := r.Update(ctx, &ws); err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
		// Workspace is being deleted
		ws.Status.Phase = v1alpha1.WorkspacePhaseDeleting
		if err := r.Status().Update(ctx, &ws); err != nil {
			return ctrl.Result{}, err
		}

		// Cease reconciliation
		return r.success(ctx, &ws)
	}

	// Manage RBAC role for workspace
	var role rbacv1.Role
	if err := r.Get(ctx, req.NamespacedName, &role); err != nil {
		if errors.IsNotFound(err) {
			role := *newRoleForWS(&ws)

			if err := controllerutil.SetControllerReference(&ws, &role, r.Scheme); err != nil {
				log.Error(err, "unable to set role ownership")
				return ctrl.Result{}, err
			}

			if err = r.Create(ctx, &role); err != nil {
				log.Error(err, "unable to create role")
				return ctrl.Result{}, err
			}
		} else if err != nil {
			log.Error(err, "unable to get role")
			return ctrl.Result{}, err
		}
	}

	// Manage RBAC role binding for workspace
	var binding rbacv1.RoleBinding
	if err := r.Get(ctx, req.NamespacedName, &binding); err != nil {
		if errors.IsNotFound(err) {
			binding := *newRoleBindingForWS(&ws)

			if err := controllerutil.SetControllerReference(&ws, &binding, r.Scheme); err != nil {
				log.Error(err, "unable to set binding ownership")
				return ctrl.Result{}, err
			}

			if err = r.Create(ctx, &binding); err != nil {
				log.Error(err, "unable to create binding")
				return ctrl.Result{}, err
			}
		} else if err != nil {
			log.Error(err, "unable to get binding")
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
	var podCreated bool
	if err := r.Get(ctx, types.NamespacedName{Namespace: req.Namespace, Name: ws.PodName()}, &pod); err != nil {
		if errors.IsNotFound(err) {
			pod, err := workspacePod(&ws, r.Image)
			if err != nil {
				log.Error(err, "unable to construct pod")
				return ctrl.Result{}, err
			}

			if err := controllerutil.SetControllerReference(&ws, pod, r.Scheme); err != nil {
				log.Error(err, "unable to set pod ownership")
				return ctrl.Result{}, err
			}

			if err = r.Create(ctx, pod); err != nil {
				log.Error(err, "unable to create pod")
				return ctrl.Result{}, err
			}
			podCreated = true

		} else if err != nil {
			log.Error(err, "unable to get pod")
			return ctrl.Result{}, err
		}
	}

	if podCreated {
		// Brand new pod so update status and end reconcile early
		ws.Status.Phase = v1alpha1.WorkspacePhaseInitializing

		if err := r.Status().Update(ctx, &ws); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// Set workspace phase status
	if err := r.setPhase(ctx, &ws, &pod); err != nil {
		return ctrl.Result{}, err
	}

	// Update run queue
	if err := updateQueue(r.Client, &ws); err != nil {
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
				return ctrl.Result{}, err
			}
		}
	}

	return r.success(ctx, &ws)
}

// success marks a successful reconcile
func (r *WorkspaceReconciler) success(ctx context.Context, ws *v1alpha1.Workspace) (ctrl.Result, error) {
	if !ws.Status.Reconciled {
		ws.Status.Reconciled = true
		if err := r.Status().Update(ctx, ws); err != nil {
			return ctrl.Result{}, err
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

func newPVCForWS(ws *v1alpha1.Workspace) *corev1.PersistentVolumeClaim {
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
					corev1.ResourceStorage: resource.MustParse(ws.Spec.Cache.Size),
				},
			},
			StorageClassName: ws.Spec.Cache.StorageClass,
		},
	}

	// Set etok's common labels
	labels.SetCommonLabels(pvc)
	// Permit filtering etok resources by component
	labels.SetLabel(pvc, labels.WorkspaceComponent)

	return pvc
}

func newRoleForWS(ws *v1alpha1.Workspace) *rbacv1.Role {
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ws.Name,
			Namespace: ws.Namespace,
		},
		Rules: []rbacv1.PolicyRule{
			// Runner may need to persist a lock file to a new config map
			{
				Resources: []string{"configmaps"},
				Verbs:     []string{"create"},
				APIGroups: []string{""},
			},
			// ...and the runner specifies the run resource as owner of said
			// config map, so it needs to retrieve run resource metadata as well
			{
				Resources: []string{"runs"},
				Verbs:     []string{"get"},
				APIGroups: []string{"etok.dev"},
			},
			// Terraform state backend mgmt
			{
				Resources: []string{"secrets"},
				Verbs:     []string{"list", "create", "get", "delete", "patch", "update"},
				APIGroups: []string{""},
			},
			// Terraform state backend mgmt
			{
				Resources: []string{"leases"},
				Verbs:     []string{"list", "create", "get", "delete", "patch", "update"},
				APIGroups: []string{"coordination.k8s.io"},
			},
		},
	}

	// Set etok's common labels
	labels.SetCommonLabels(role)
	// Permit filtering etok resources by component
	labels.SetLabel(role, labels.WorkspaceComponent)

	return role
}

func newRoleBindingForWS(ws *v1alpha1.Workspace) *rbacv1.RoleBinding {
	binding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ws.Name,
			Namespace: ws.Namespace,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      ws.Spec.ServiceAccountName,
				Namespace: ws.Namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "Role",
			Name:     ws.Name,
			APIGroup: "rbac.authorization.k8s.io",
		},
	}

	// Set etok's common labels
	labels.SetCommonLabels(binding)
	// Permit filtering etok resources by component
	labels.SetLabel(binding, labels.WorkspaceComponent)

	return binding
}

func (r *WorkspaceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	blder := ctrl.NewControllerManagedBy(mgr)

	// Watch for changes to primary resource Workspace
	blder = blder.For(&v1alpha1.Workspace{})

	// Watch for changes to secondary resource PVCs and requeue the owner Workspace
	blder = blder.Owns(&corev1.PersistentVolumeClaim{})

	// Watch owned pods
	blder = blder.Owns(&corev1.Pod{})

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
