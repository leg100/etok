package workspace

import (
	"context"
	"fmt"
	"reflect"

	"github.com/leg100/stok/constants"
	"github.com/leg100/stok/crdinfo"
	"github.com/leg100/stok/pkg/apis"
	v1alpha1 "github.com/leg100/stok/pkg/apis/stok/v1alpha1"
	"github.com/leg100/stok/pkg/apis/stok/v1alpha1/command"
	"github.com/operator-framework/operator-sdk/pkg/status"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_workspace")
var someIndexer client.FieldIndexer

// Add creates a new Workspace Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileWorkspace{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("workspace-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	_ = mgr.GetFieldIndexer().IndexField(&v1alpha1.Workspace{}, "spec.serviceAccountName", func(o runtime.Object) []string {
		sa := o.(*v1alpha1.Workspace).Spec.ServiceAccountName
		if sa == "" {
			return nil
		}
		return []string{sa}
	})

	_ = mgr.GetFieldIndexer().IndexField(&v1alpha1.Workspace{}, "spec.secretName", func(o runtime.Object) []string {
		sa := o.(*v1alpha1.Workspace).Spec.SecretName
		if sa == "" {
			return nil
		}
		return []string{sa}
	})

	// Watch for changes to primary resource Workspace
	err = c.Watch(&source.Kind{Type: &v1alpha1.Workspace{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to secondary resource PVCs and requeue the owner Workspace
	err = c.Watch(&source.Kind{Type: &corev1.PersistentVolumeClaim{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &v1alpha1.Workspace{},
	})
	if err != nil {
		return err
	}

	// Watch for changes to service accounts and secrets, because they may affect the functionality
	// of a Workspace (e.g. the deletion of a service account)
	err = c.Watch(&source.Kind{Type: &corev1.ServiceAccount{}}, &handler.EnqueueRequestsFromMapFunc{
		ToRequests: handler.ToRequestsFunc(func(a handler.MapObject) []reconcile.Request {
			var reqs []reconcile.Request
			wsList := &v1alpha1.WorkspaceList{}
			filter := client.MatchingFields{"spec.serviceAccountName": a.Meta.GetName()}
			err = r.(*ReconcileWorkspace).client.List(context.TODO(), wsList, client.InNamespace(a.Meta.GetNamespace()), filter)
			if err != nil {
				return reqs
			}
			meta.EachListItem(wsList, func(ws runtime.Object) error {
				reqs = append(reqs, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      ws.(*v1alpha1.Workspace).GetName(),
						Namespace: a.Meta.GetNamespace(),
					},
				})
				return nil
			})
			return reqs
		}),
	})
	if err != nil {
		return err
	}

	err = c.Watch(&source.Kind{Type: &corev1.Secret{}}, &handler.EnqueueRequestsFromMapFunc{
		ToRequests: handler.ToRequestsFunc(func(a handler.MapObject) []reconcile.Request {
			var reqs []reconcile.Request
			wsList := &v1alpha1.WorkspaceList{}
			filter := client.MatchingFields{"spec.secretName": a.Meta.GetName()}
			err = r.(*ReconcileWorkspace).client.List(context.TODO(), wsList, client.InNamespace(a.Meta.GetNamespace()), filter)
			if err != nil {
				return reqs
			}
			meta.EachListItem(wsList, func(ws runtime.Object) error {
				reqs = append(reqs, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      ws.(*v1alpha1.Workspace).GetName(),
						Namespace: a.Meta.GetNamespace(),
					},
				})
				return nil
			})
			return reqs
		}),
	})
	if err != nil {
		return err
	}

	s := mgr.GetScheme()
	apis.AddToScheme(s)
	// Watch for changes to command resources and requeue the associated Workspace.
	for _, crd := range crdinfo.Inventory {
		o, err := s.New(crd.GroupVersionKind())
		if err != nil {
			return err
		}

		err = c.Watch(&source.Kind{Type: o}, &handler.EnqueueRequestsFromMapFunc{
			ToRequests: handler.ToRequestsFunc(func(a handler.MapObject) []reconcile.Request {
				cmd := a.Object.(command.Interface)
				if ws, ok := cmd.GetLabels()["workspace"]; ok {
					return []reconcile.Request{
						{
							NamespacedName: types.NamespacedName{
								Name:      ws,
								Namespace: a.Meta.GetNamespace(),
							},
						},
					}
				}
				return []reconcile.Request{}
			}),
		})
		if err != nil {
			return err
		}
	}

	return nil
}

// blank assignment to verify that ReconcileWorkspace implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileWorkspace{}

// ReconcileWorkspace reconciles a Workspace object
type ReconcileWorkspace struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a Workspace object and makes changes based on the state read
// and what is in the Workspace.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileWorkspace) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.V(1).Info("Reconciling Workspace")

	// Fetch the Workspace instance
	instance := &v1alpha1.Workspace{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		reqLogger.Error(err, "Error retrieving workspace")
		return reconcile.Result{}, err
	}

	// Because it is a required attribute we need to set the queue status to an empty array if it
	// is not already set
	if instance.Status.Queue == nil {
		instance.Status.Queue = []string{}
	}

	pvc := newPVCForCR(instance)

	// Set Workspace instance as the owner and controller
	if err := controllerutil.SetControllerReference(instance, pvc, r.scheme); err != nil {
		reqLogger.Error(err, "Error setting controller reference on PVC")
		return reconcile.Result{}, err
	}

	// Check if this PVC already exists
	// NOTE: Once created, changes to the PVC, such as the size or the storage class name, are
	// ignored. The PVC has to be manually deleted and then let the controller re-create it.
	found := &corev1.PersistentVolumeClaim{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: pvc.Name, Namespace: pvc.Namespace}, found)
	if errors.IsNotFound(err) {
		reqLogger.Info("Creating a new PVC", "PVC.Namespace", pvc.Namespace, "PVC.Name", pvc.Name)
		if err = r.client.Create(context.TODO(), pvc); err != nil {
			reqLogger.Error(err, "Error creating PVC")
			return reconcile.Result{}, err
		}
	} else if err != nil {
		reqLogger.Error(err, "Error retrieving PVC")
		return reconcile.Result{}, err
	}

	// Check ServiceAccount exists (if specified)
	if instance.Spec.ServiceAccountName != "" {
		serviceAccountNamespacedName := types.NamespacedName{Name: instance.Spec.ServiceAccountName, Namespace: request.Namespace}
		err = r.client.Get(context.TODO(), serviceAccountNamespacedName, &corev1.ServiceAccount{})
		if errors.IsNotFound(err) {
			instance.Status.Conditions.SetCondition(status.Condition{
				Type:    v1alpha1.ConditionHealthy,
				Status:  corev1.ConditionFalse,
				Reason:  v1alpha1.ReasonMissingResource,
				Message: "ServiceAccount resource not found",
			})
			if err = r.client.Status().Update(context.TODO(), instance); err != nil {
				return reconcile.Result{}, fmt.Errorf("Setting healthy condition: %w", err)
			}
			// Pointless proceeding any further or requeuing a request (the service account watch will
			// take care of triggering a request)
			return reconcile.Result{}, nil
		} else if err != nil {
			return reconcile.Result{}, err
		}
	}

	// Flag success if Secret is either:
	// (a) unspecified and thus not required
	// (b) specified and successfully found
	if instance.Spec.SecretName != "" {
		secretNamespacedName := types.NamespacedName{Name: instance.Spec.SecretName, Namespace: request.Namespace}
		err = r.client.Get(context.TODO(), secretNamespacedName, &corev1.Secret{})
		if errors.IsNotFound(err) {
			instance.Status.Conditions.SetCondition(status.Condition{
				Type:    v1alpha1.ConditionHealthy,
				Status:  corev1.ConditionFalse,
				Reason:  v1alpha1.ReasonMissingResource,
				Message: "Secret resource not found",
			})
			if err = r.client.Status().Update(context.TODO(), instance); err != nil {
				return reconcile.Result{}, fmt.Errorf("Setting healthy condition: %w", err)
			}
			// Pointless proceeding any further or requeuing a request (the secret watch will
			// take care of triggering a request)
			return reconcile.Result{}, nil
		} else if err != nil {
			return reconcile.Result{}, err
		}
	}

	// Set Healthy Condition since all pre-requisities satisfied
	instance.Status.Conditions.SetCondition(status.Condition{
		Type:    v1alpha1.ConditionHealthy,
		Status:  corev1.ConditionTrue,
		Reason:  v1alpha1.ReasonAllResourcesFound,
		Message: "All prerequisite resources found",
	})
	if err := r.client.Status().Update(context.TODO(), instance); err != nil {
		return reconcile.Result{}, fmt.Errorf("Setting healthy condition: %w", err)
	}

	// Fetch list of commands that belong to this workspace (its workspace label specifies this workspace)
	var cmdList []command.Interface
	// Fetch and append each type of command to cmdList
	for _, crd := range crdinfo.Inventory {
		ccList, err := r.scheme.New(crd.GroupVersionKindList())
		if err != nil {
			return reconcile.Result{}, err
		}

		err = r.client.List(context.TODO(), ccList, client.InNamespace(request.Namespace), client.MatchingLabels{
			"workspace": request.Name,
		})
		if err != nil {
			return reconcile.Result{}, err
		}

		meta.EachListItem(ccList, func(o runtime.Object) error {
			cmdList = append(cmdList, o.(command.Interface))
			return nil
		})
	}

	// Filter out completed commands
	n := 0
	for _, cmd := range cmdList {
		if cond := cmd.GetConditions().IsTrueFor(v1alpha1.ConditionCompleted); !cond {
			cmdList[n] = cmd
			n++
		}
	}
	cmdList = cmdList[:n]

	// Filter out completed/deleted commands from existing queue, building new queue
	newQueue := []string{}
	for _, cmd := range instance.Status.Queue {
		if i := cmdListMatchingName(cmdList, cmd); i > -1 {
			// add to new queue
			newQueue = append(newQueue, cmd)
			// remove from cmd list
			cmdList = append(cmdList[:i], cmdList[i+1:]...)
		}
		// cmd not found, leave it out of new queue
	}
	// Append remainder of commands
	newQueue = append(newQueue, cmdListNames(cmdList)...)

	// update status if queue has changed
	if !reflect.DeepEqual(newQueue, instance.Status.Queue) {
		reqLogger.Info("Queue updated", "Old", fmt.Sprintf("%#v", instance.Status.Queue), "New", fmt.Sprintf("%#v", newQueue))
		instance.Status.Queue = newQueue
		if err := r.client.Status().Update(context.TODO(), instance); err != nil {
			return reconcile.Result{}, fmt.Errorf("Failed to update queue status: %w", err)
		}
	}

	return reconcile.Result{}, nil
}

func newPVCForCR(cr *v1alpha1.Workspace) *corev1.PersistentVolumeClaim {
	labels := map[string]string{
		"app": cr.Name,
	}
	size := constants.WorkspaceCacheSize
	if cr.Spec.Cache.Size != "" {
		size = cr.Spec.Cache.Size
	}
	pvc := corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name,
			Namespace: cr.Namespace,
			Labels:    labels,
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

	if cr.Spec.Cache.StorageClass != "" {
		pvc.Spec.StorageClassName = &cr.Spec.Cache.StorageClass
	}

	return &pvc
}

func cmdListMatchingName(cmdList []command.Interface, name string) int {
	for i, cmd := range cmdList {
		if cmd.GetName() == name {
			return i
		}
	}
	return -1
}

func cmdListNames(cmdList []command.Interface) []string {
	names := []string{}
	for _, cmd := range cmdList {
		names = append(names, cmd.GetName())
	}
	return names
}
