package command

import (
	"context"
	"fmt"

	"github.com/leg100/stok/constants"
	"github.com/leg100/stok/crdinfo"
	v1alpha1 "github.com/leg100/stok/pkg/apis/stok/v1alpha1"
	"github.com/leg100/stok/pkg/controller/dependents"
	"github.com/leg100/stok/util/slice"
	"github.com/operator-framework/operator-sdk/pkg/status"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type CommandList interface {
	GetItems() []Command
}

type Command interface {
	runtime.Object
	metav1.Object

	GetConditions() *status.Conditions
	SetConditions(status.Conditions)
	GetArgs() []string
	SetArgs([]string)
}

func Reconcile(client client.Client, scheme *runtime.Scheme, request reconcile.Request, command Command, crdname string) (reconcile.Result, error) {
	log := logf.Log.WithName("controller_" + crdname)
	_ = log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)

	crd, ok := crdinfo.Inventory[crdname]
	if !ok {
		return reconcile.Result{}, fmt.Errorf("Could not find crd '%s'", crdname)
	}

	err := client.Get(context.TODO(), request.NamespacedName, command)
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

	//TODO: we really shouldn't be using a label for this but a spec field instead
	workspaceName, ok := command.GetLabels()["workspace"]
	if !ok {
		return reconcile.Result{}, fmt.Errorf("could not find workspace label")
	}

	// Fetch its Workspace object
	workspace := &v1alpha1.Workspace{}
	err = client.Get(context.TODO(), types.NamespacedName{Name: workspaceName, Namespace: request.Namespace}, workspace)
	if err != nil {
		_, err := updateCondition(client, command, "WorkspaceReady", corev1.ConditionFalse, "WorkspaceNotFound", fmt.Sprintf("Workspace '%s' not found", workspaceName))
		return reconcile.Result{}, err
	}

	// Get workspace's secret resource if one exists
	secret := &corev1.Secret{}
	err = client.Get(context.TODO(), types.NamespacedName{Name: workspace.Spec.SecretName, Namespace: request.Namespace}, secret)
	// ignore not found errors because a secret is optional
	if err != nil && !errors.IsNotFound(err) {
		return reconcile.Result{}, err
	}

	labels := map[string]string{
		"app":       "stok",
		"command":   command.GetName(),
		"workspace": workspace.Name,
	}
	dm := dependents.NewDependentMgr(command, client, scheme, request.Name, request.Namespace, labels)

	// Create pod
	pod := &CommandPod{
		resource:           command.GetName(),
		workspaceName:      workspace.Name,
		serviceAccountName: constants.ServiceAccountName,
		secretName:         secret.GetName(),
		configMap:          command.GetName(),
		pvcName:            workspace.Name,
		args:               command.GetArgs(),
		crd:                crd,
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
		updated, err := updateCondition(client, command, "Completed", corev1.ConditionTrue, "Pod"+string(phase), "")
		if err != nil || updated {
			return reconcile.Result{}, err
		}
	}

	// Check if client has told us they're ready and set condition accordingly
	if val, ok := command.GetAnnotations()["stok.goalspike.com/client"]; ok && val == "Ready" {
		updated, err := updateCondition(client, command, "ClientReady", corev1.ConditionTrue, "ClientReceivingLogs", "Logs are being streamed to the client")
		if err != nil || updated {
			return reconcile.Result{}, err
		}
	}

	// Check workspace queue position
	pos := slice.StringIndex(workspace.Status.Queue, command.GetName())
	switch {
	case pos == 0:
		_, err = updateCondition(client, command, "WorkspaceReady", corev1.ConditionTrue, "Active", "Front of workspace queue")
	case pos < 0:
		_, err = updateCondition(client, command, "WorkspaceReady", corev1.ConditionFalse, "Unenqueued", "Waiting to be added to the workspace queue")
	default:
		_, err = updateCondition(client, command, "WorkspaceReady", corev1.ConditionFalse, "Queued", fmt.Sprintf("Waiting in workspace queue; position: %d", pos))
	}
	return reconcile.Result{}, err
}

func updateCondition(client client.Client, command Command, conditionType string, conditionStatus corev1.ConditionStatus, reason string, msg string) (bool, error) {
	c := status.Condition{
		Type:    status.ConditionType(conditionType),
		Status:  conditionStatus,
		Reason:  status.ConditionReason(reason),
		Message: msg,
	}
	cc := command.GetConditions().GetCondition(c.Type)

	// only update if this is a new condition status or
	// any of the condition attributes have changed
	if cc == nil || cc.Status != c.Status || cc.Reason != c.Reason || cc.Message != c.Message {
		_ = command.GetConditions().SetCondition(c)
		return true, client.Status().Update(context.TODO(), command)
	} else {
		return false, nil
	}
}
