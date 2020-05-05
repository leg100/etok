package command

import (
	"context"
	"fmt"

	v1alpha1 "github.com/leg100/stok/pkg/apis/stok/v1alpha1"
	"github.com/leg100/stok/pkg/controller/dependents"
	"github.com/leg100/stok/util/slice"
	"github.com/operator-framework/operator-sdk/pkg/status"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_command")

// Add creates a new Command Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileCommand{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("command-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Command
	err = c.Watch(&source.Kind{Type: &v1alpha1.Command{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to secondary resource Pods and requeue the owner Command
	err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &v1alpha1.Command{},
	})
	if err != nil {
		return err
	}

	// Watch for changes to resource Workspace and requeue the associated Commands
	err = c.Watch(&source.Kind{Type: &v1alpha1.Workspace{}}, &handler.EnqueueRequestsFromMapFunc{
		ToRequests: handler.ToRequestsFunc(func(a handler.MapObject) []reconcile.Request {
			rc := r.(*ReconcileCommand)
			cmdList := &v1alpha1.CommandList{}
			err = rc.client.List(context.TODO(), cmdList, client.InNamespace(a.Meta.GetNamespace()), client.MatchingLabels{
				"workspace": a.Meta.GetName(),
			})
			if err != nil {
				return []reconcile.Request{}
			}

			rr := []reconcile.Request{}
			for _, cmd := range cmdList.Items {
				rr = append(rr, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      cmd.GetName(),
						Namespace: cmd.GetNamespace(),
					},
				})
			}
			return rr
		}),
	})
	if err != nil {
		return err
	}
	return nil
}

// blank assignment to verify that ReconcileCommand implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileCommand{}

// ReconcileCommand reconciles a Command object
type ReconcileCommand struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a Command object and makes changes based on the state read
// and what is in the Command.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileCommand) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.V(1).Info("Reconciling Command")

	// Fetch the Command instance
	c := &v1alpha1.Command{}
	err := r.client.Get(context.TODO(), request.NamespacedName, c)
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

	// Fetch its Workspace object
	workspace := &v1alpha1.Workspace{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: c.Labels["workspace"], Namespace: request.Namespace}, workspace)
	if err != nil {
		_, err := r.updateCondition(c, "WorkspaceReady", corev1.ConditionFalse, "WorkspaceNotFound", fmt.Sprintf("Workspace '%s' not found", c.Labels["workspace"]))
		return reconcile.Result{}, err
	}

	// Get workspace's secret resource
	secret := &corev1.Secret{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: workspace.Spec.SecretName, Namespace: request.Namespace}, secret)
	if err != nil {
		_, err := r.updateCondition(c, "WorkspaceReady", corev1.ConditionFalse, "SecretNotFound", fmt.Sprintf("Secret '%s' not found", secret.GetName()))
		return reconcile.Result{}, err
	}

	labels := map[string]string{
		"app":       "stok",
		"command":   c.Name,
		"workspace": workspace.Name,
	}
	dm := dependents.NewDependentMgr(c, r.client, r.scheme, request.Name, request.Namespace, labels)

	// Create service account
	if _, err := dm.GetOrCreate(&CommandServiceAccount{}); err != nil {
		return reconcile.Result{}, err
	}

	// Create role
	if _, err := dm.GetOrCreate(&CommandRole{commandName: c.Name}); err != nil {
		return reconcile.Result{}, err
	}

	//// Create rolebinding
	rb := CommandRoleBinding{serviceAccountName: c.Name, roleName: c.Name}
	if _, err := dm.GetOrCreate(&rb); err != nil {
		return reconcile.Result{}, err
	}

	// Create pod
	pod := &CommandPod{
		commandName:        c.Name,
		workspaceName:      workspace.Name,
		serviceAccountName: c.Name,
		secretName:         secret.Name,
		configMap:          c.Spec.ConfigMap,
		configMapKey:       c.Spec.ConfigMapKey,
		pvcName:            workspace.Name,
		command:            c.Spec.Command,
		args:               c.Spec.Args,
	}
	created, err := dm.GetOrCreate(pod)
	if err != nil {
		return reconcile.Result{}, err
	}
	if created {
		return reconcile.Result{Requeue: true}, nil
	}

	// Signal pod completion to workspace
	phase := pod.Status.Phase
	if phase == corev1.PodSucceeded || phase == corev1.PodFailed {
		updated, err := r.updateCondition(c, "Completed", corev1.ConditionTrue, "Pod"+string(phase), "")
		if err != nil || updated {
			return reconcile.Result{}, err
		}
	}

	// Check if client has told us they're ready and set condition accordingly
	if val, ok := c.Annotations["stok.goalspike.com/client"]; ok && val == "Ready" {
		updated, err := r.updateCondition(c, "ClientReady", corev1.ConditionTrue, "ClientReceivingLogs", "Logs are being streamed to the client")
		if err != nil || updated {
			return reconcile.Result{}, err
		}
	}

	// Check workspace queue position
	pos := slice.StringIndex(workspace.Status.Queue, c.GetName())
	switch {
	case pos == 0:
		_, err = r.updateCondition(c, "WorkspaceReady", corev1.ConditionTrue, "Active", "Front of workspace queue")
	case pos < 0:
		_, err = r.updateCondition(c, "WorkspaceReady", corev1.ConditionFalse, "Unenqueued", "Waiting to be added to the workspace queue")
	default:
		_, err = r.updateCondition(c, "WorkspaceReady", corev1.ConditionFalse, "Queued", fmt.Sprintf("Waiting in workspace queue; position: %d", pos))
	}
	return reconcile.Result{}, err
}

func (r *ReconcileCommand) updateCondition(command *v1alpha1.Command, conditionType string, conditionStatus corev1.ConditionStatus, reason string, msg string) (bool, error) {
	c := status.Condition{
		Type:    status.ConditionType(conditionType),
		Status:  conditionStatus,
		Reason:  status.ConditionReason(reason),
		Message: msg,
	}
	cc := command.Status.Conditions.GetCondition(c.Type)

	// only update if this is a new condition status or
	// any of the condition attributes have changed
	if cc == nil || cc.Status != c.Status || cc.Reason != c.Reason || cc.Message != c.Message {
		_ = command.Status.Conditions.SetCondition(c)
		return true, r.client.Status().Update(context.TODO(), command)
	} else {
		return false, nil
	}
}
