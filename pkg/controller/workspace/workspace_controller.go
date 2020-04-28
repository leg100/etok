package workspace

import (
	"context"
	"reflect"

	terraformv1alpha1 "github.com/leg100/stok/pkg/apis/terraform/v1alpha1"
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

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

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

	//_ = mgr.GetFieldIndexer().IndexField(&terraformv1alpha1.Command{}, "Spec.Workspace", func(o runtime.Object) []string {
	//	workspace := o.(*terraformv1alpha1.Command).Spec.Workspace
	//	if workspace == "" {
	//		return nil
	//	}
	//	return []string{workspace}
	//})

	// Watch for changes to primary resource Workspace
	err = c.Watch(&source.Kind{Type: &terraformv1alpha1.Workspace{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Workspace
	err = c.Watch(&source.Kind{Type: &terraformv1alpha1.Workspace{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to secondary resource PVCs and requeue the owner Workspace
	err = c.Watch(&source.Kind{Type: &corev1.PersistentVolumeClaim{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &terraformv1alpha1.Workspace{},
	})
	if err != nil {
		return err
	}

	// Watch for changes to resource Command and requeue the associated Workspace
	err = c.Watch(&source.Kind{Type: &terraformv1alpha1.Command{}}, &handler.EnqueueRequestsFromMapFunc{
		ToRequests: handler.ToRequestsFunc(func(a handler.MapObject) []reconcile.Request {
			command := a.Object.(*terraformv1alpha1.Command)
			return []reconcile.Request{
				{
					NamespacedName: types.NamespacedName{
						Name:      command.Labels["workspace"],
						Namespace: a.Meta.GetNamespace(),
					},
				},
			}
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
	reqLogger.Info("Reconciling Workspace")

	// Fetch the Workspace instance
	instance := &terraformv1alpha1.Workspace{}
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

	// process existing command queue
	newQueue := []string{}
	for _, cmdName := range instance.Status.Queue {
		cmd := &terraformv1alpha1.Command{}
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: cmdName, Namespace: request.Namespace}, cmd)
		if err != nil && errors.IsNotFound(err) {
			reqLogger.Info("Command not found; removing from queue", "Command.Name", cmdName)
			continue
		} else if err != nil {
			return reconcile.Result{}, err
		} else {
			if cmd.Status.Conditions.IsTrueFor(status.ConditionType("Completed")) {
				reqLogger.Info("Command completed; removing from queue", "Command.Name", cmdName)
				continue
			} else {
				newQueue = append(newQueue, cmdName)
			}
		}
	}

	// process command resources
	cmdList := &terraformv1alpha1.CommandList{}
	err = r.client.List(context.TODO(), cmdList, client.InNamespace(request.Namespace), client.MatchingLabels{
		"workspace": request.Name,
	})
	if err != nil {
		return reconcile.Result{}, err
	}
	for _, cmd := range cmdList.Items {
		if cmd.Status.Conditions.IsTrueFor(status.ConditionType("Completed")) {
			continue
		}
		if cmdIsQueued(&cmd, instance.Status.Queue) {
			continue
		}
		reqLogger.Info("Adding command to queue", "Command.Name", cmd.GetName())
		newQueue = append(newQueue, cmd.GetName())
	}

	// update status if queue has changed
	if !reflect.DeepEqual(newQueue, instance.Status.Queue) {
		instance.Status.Queue = newQueue
		err := r.client.Status().Update(context.TODO(), instance)
		if err != nil {
			reqLogger.Error(err, "Failed to update queue status")
			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{}, nil
}

func cmdIsQueued(cmd *terraformv1alpha1.Command, queue []string) bool {
	for _, qCmd := range queue {
		if qCmd == cmd.GetName() {
			return true
		}
	}
	return false
}

func newPVCForCR(cr *terraformv1alpha1.Workspace) *corev1.PersistentVolumeClaim {
	labels := map[string]string{
		"app": cr.Name,
	}
	return &corev1.PersistentVolumeClaim{
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
					corev1.ResourceStorage: resource.MustParse("1Gi"),
				},
			},
		},
	}
}
