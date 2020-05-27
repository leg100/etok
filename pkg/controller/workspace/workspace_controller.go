package workspace

import (
	"context"
	"fmt"
	"reflect"

	"github.com/leg100/stok/constants"
	v1alpha1 "github.com/leg100/stok/pkg/apis/stok/v1alpha1"
	"github.com/operator-framework/operator-sdk/pkg/status"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
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

	//_ = mgr.GetFieldIndexer().IndexField(&v1alpha1.Command{}, "Spec.Workspace", func(o runtime.Object) []string {
	//	workspace := o.(*v1alpha1.Command).Spec.Workspace
	//	if workspace == "" {
	//		return nil
	//	}
	//	return []string{workspace}
	//})

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

	// Watch for changes to resource Pod and requeue the associated Workspace.
	// Filter out pods with irrelevant labels.
	err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestsFromMapFunc{
		ToRequests: handler.ToRequestsFunc(func(a handler.MapObject) []reconcile.Request {
			pod := a.Object.(*corev1.Pod)
			if app, ok := pod.GetLabels()["app"]; ok && app == "stok" {
				if workspace, ok := pod.GetLabels()["workspace"]; ok {
					return []reconcile.Request{
						{
							NamespacedName: types.NamespacedName{
								Name:      workspace,
								Namespace: a.Meta.GetNamespace(),
							},
						},
					}
				}
			}
			return []reconcile.Request{}
		}),
	})
	if err != nil {
		return err
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
		return reconcile.Result{}, err
	}

	pvc := newPVCForCR(instance)

	// Set Workspace instance as the owner and controller
	if err := controllerutil.SetControllerReference(instance, pvc, r.scheme); err != nil {
		return reconcile.Result{}, err
	}

	// Check if this PVC already exists
	// NOTE: Once created, changes to the PVC, such as the size or the storage class name, are
	// ignored. The PVC has to be manually deleted and then let the controller re-create it.
	found := &corev1.PersistentVolumeClaim{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: pvc.Name, Namespace: pvc.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating a new PVC", "PVC.Namespace", pvc.Namespace, "PVC.Name", pvc.Name)
		err = r.client.Create(context.TODO(), pvc)
		if err != nil {
			return reconcile.Result{}, err
		}
	} else if err != nil {
		return reconcile.Result{}, err
	}

	// Check specified ServiceAccount exists
	serviceAccount := &corev1.ServiceAccount{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: instance.Spec.ServiceAccountName, Namespace: request.Namespace}, serviceAccount)
	if err != nil && errors.IsNotFound(err) {
		updated := instance.Status.Conditions.SetCondition(serviceAccountNotFoundCondition(instance.Spec.ServiceAccountName))
		if updated {
			if err = r.client.Status().Update(context.TODO(), instance); err != nil {
				return reconcile.Result{}, err
			}
		}
	} else if err != nil {
		return reconcile.Result{}, err
	} else {
		updated := instance.Status.Conditions.SetCondition(serviceAccountFoundCondition(instance.Spec.ServiceAccountName))
		if updated {
			if err = r.client.Status().Update(context.TODO(), instance); err != nil {
				return reconcile.Result{}, err
			}
		}
	}

	// Check specified Secret exists
	secret := &corev1.Secret{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: instance.Spec.SecretName, Namespace: request.Namespace}, secret)
	if err != nil && errors.IsNotFound(err) {
		updated := instance.Status.Conditions.SetCondition(secretNotFoundCondition(instance.Spec.SecretName))
		if updated {
			if err = r.client.Status().Update(context.TODO(), instance); err != nil {
				return reconcile.Result{}, err
			}
		}
	} else if err != nil {
		return reconcile.Result{}, err
	} else {
		updated := instance.Status.Conditions.SetCondition(secretFoundCondition(instance.Spec.SecretName))
		if updated {
			if err = r.client.Status().Update(context.TODO(), instance); err != nil {
				return reconcile.Result{}, err
			}
		}
	}

	// Set Phase
	var phase string
	if allConditionsTrue(instance.Status.Conditions) {
		phase = "Ready"
	} else {
		phase = "Unready"
	}
	if instance.Status.Phase != phase {
		instance.Status.Phase = phase
		if err = r.client.Status().Update(context.TODO(), instance); err != nil {
			return reconcile.Result{}, err
		}
	}

	// fetch list of (relevant) pods
	podList := corev1.PodList{}
	err = r.client.List(context.TODO(), &podList, client.InNamespace(request.Namespace), client.MatchingLabels{
		"app":       "stok",
		"workspace": request.Name,
	})
	if err != nil {
		return reconcile.Result{}, err
	}

	// filter running pods
	n := 0
	for _, pod := range podList.Items {
		if pod.Status.Phase == corev1.PodRunning {
			podList.Items[n] = pod
			n++
		}
	}
	podList.Items = podList.Items[:n]

	// Filter out completed/deleted commands from existing queue, building new queue
	newQueue := []string{}
	for _, cmd := range instance.Status.Queue {
		if i := podListMatchingName(&podList, cmd); i > -1 {
			// add to new queue
			newQueue = append(newQueue, cmd)
			// remove from pod list so we don't include it again later
			podList.Items = append(podList.Items[:i], podList.Items[i+1:]...)
		}
		// cmd not found, leave it out of new queue
	}
	// Append remainder of pods
	newQueue = append(newQueue, podListNames(&podList)...)

	// update status if queue has changed
	if !reflect.DeepEqual(newQueue, instance.Status.Queue) {
		reqLogger.Info("Queue updated", "Old", instance.Status.Queue, "New", newQueue)
		instance.Status.Queue = newQueue
		err := r.client.Status().Update(context.TODO(), instance)
		if err != nil {
			reqLogger.Error(err, "Failed to update queue status")
			return reconcile.Result{}, err
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

func podListMatchingName(podList *corev1.PodList, name string) int {
	for i, pod := range podList.Items {
		if pod.GetName() == name {
			return i
		}
	}
	return -1
}

func podListNames(podList *corev1.PodList) []string {
	names := []string{}
	for _, pod := range podList.Items {
		names = append(names, pod.GetName())
	}
	return names
}

func serviceAccountNotFoundCondition(serviceAccountName string) status.Condition {
	return status.Condition{
		Type:    status.ConditionType("ServiceAccountReady"),
		Status:  corev1.ConditionFalse,
		Reason:  status.ConditionReason("ServiceAccountNotFound"),
		Message: fmt.Sprintf("ServiceAccount '%s' not found", serviceAccountName),
	}
}

func serviceAccountFoundCondition(serviceAccountName string) status.Condition {
	return status.Condition{
		Type:    status.ConditionType("ServiceAccountReady"),
		Status:  corev1.ConditionTrue,
		Reason:  status.ConditionReason("ServiceAccountFound"),
		Message: fmt.Sprintf("ServiceAccount '%s' found", serviceAccountName),
	}
}

func secretNotFoundCondition(secretName string) status.Condition {
	return status.Condition{
		Type:    status.ConditionType("SecretReady"),
		Status:  corev1.ConditionFalse,
		Reason:  status.ConditionReason("SecretNotFound"),
		Message: fmt.Sprintf("Secret '%s' not found", secretName),
	}
}

func secretFoundCondition(secretName string) status.Condition {
	return status.Condition{
		Type:    status.ConditionType("SecretReady"),
		Status:  corev1.ConditionTrue,
		Reason:  status.ConditionReason("SecretFound"),
		Message: fmt.Sprintf("Secret '%s' found", secretName),
	}
}

func allConditionsTrue(conditions status.Conditions) bool {
	for _, cond := range conditions {
		if cond.IsFalse() {
			return false
		}
	}
	return true
}
