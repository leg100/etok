package controllers

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/leg100/stok/api/command"
	v1alpha1 "github.com/leg100/stok/api/v1alpha1"
	"github.com/leg100/stok/util/slice"
	operatorstatus "github.com/operator-framework/operator-sdk/pkg/status"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

type CommandReconciler struct {
	client.Client
	C            command.Interface
	Kind         string
	ResourceType string
	Scheme       *runtime.Scheme
	Log          logr.Logger
	RunnerImage  string
}

func (r *CommandReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	reqLogger := r.Log.WithValues(r.ResourceType, req.NamespacedName)
	reqLogger.V(0).Info("Reconciling " + r.ResourceType)

	err := r.Get(context.TODO(), req.NamespacedName, r.C)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}

	// Command completed, nothing more to be done
	if r.C.GetConditions().IsTrueFor(v1alpha1.ConditionCompleted) {
		return ctrl.Result{}, nil
	}

	//TODO: we really shouldn't be using a label for this but a spec field instead
	workspaceName, ok := r.C.GetLabels()["workspace"]
	if !ok {
		// Unrecoverable error, signal completion
		r.C.GetConditions().SetCondition(operatorstatus.Condition{
			Type:    v1alpha1.ConditionCompleted,
			Status:  corev1.ConditionTrue,
			Reason:  v1alpha1.ReasonWorkspaceUnspecified,
			Message: "Error: Workspace label not set",
		})
		_ = r.Status().Update(context.TODO(), r.C)
		return ctrl.Result{}, nil
	}

	// Fetch its Workspace object
	workspace := &v1alpha1.Workspace{}
	err = r.Get(context.TODO(), types.NamespacedName{Name: workspaceName, Namespace: req.Namespace}, workspace)
	if errors.IsNotFound(err) {
		// Workspace not found, unlikely to be temporary, signal completion
		r.C.GetConditions().SetCondition(operatorstatus.Condition{
			Type:    v1alpha1.ConditionCompleted,
			Status:  corev1.ConditionTrue,
			Reason:  v1alpha1.ReasonWorkspaceNotFound,
			Message: fmt.Sprintf("Workspace '%s' not found", workspaceName),
		})
		_ = r.Status().Update(context.TODO(), r.C)
		return ctrl.Result{}, nil
	}

	// Check workspace queue position
	pos := slice.StringIndex(workspace.Status.Queue, r.C.GetName())
	// TODO: consider removing these status updates
	if pos > 0 {
		// Queued
		r.C.GetConditions().SetCondition(operatorstatus.Condition{
			Type:    v1alpha1.ConditionAttachable,
			Status:  corev1.ConditionFalse,
			Reason:  v1alpha1.ReasonQueued,
			Message: fmt.Sprintf("In workspace queue position %d", pos),
		})
		err = r.Status().Update(context.TODO(), r.C)
		return ctrl.Result{}, err
	}
	if pos < 0 {
		// Not yet queued
		r.C.GetConditions().SetCondition(operatorstatus.Condition{
			Type:    v1alpha1.ConditionAttachable,
			Status:  corev1.ConditionFalse,
			Reason:  v1alpha1.ReasonUnscheduled,
			Message: "Not yet scheduled by workspace",
		})
		err = r.Status().Update(context.TODO(), r.C)
		return ctrl.Result{}, err
	}

	// Check if client has told us they're ready and set condition accordingly
	if val, ok := r.C.GetAnnotations()["stok.goalspike.com/client"]; ok && val == "Ready" {
		r.C.GetConditions().SetCondition(operatorstatus.Condition{
			Type:    v1alpha1.ConditionClientReady,
			Status:  corev1.ConditionTrue,
			Reason:  v1alpha1.ReasonClientAttached,
			Message: "Client has attached to pod TTY",
		})
		if err = r.Status().Update(context.TODO(), r.C); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Currently scheduled to run; get or create pod
	opts := podOpts{
		workspaceName:      workspace.GetName(),
		serviceAccountName: workspace.Spec.ServiceAccountName,
		secretName:         workspace.Spec.SecretName,
		pvcName:            workspace.GetName(),
		configMapName:      r.C.GetConfigMap(),
		configMapKey:       r.C.GetConfigMapKey(),
	}
	return r.reconcilePod(req, &opts)
}

func (r *CommandReconciler) SetupWithManager(mgr ctrl.Manager) error {
	gvk := v1alpha1.SchemeGroupVersion.WithKind(r.Kind)
	o, err := r.Scheme.New(gvk)
	if err != nil {
		return err
	}

	oList, err := r.Scheme.New(v1alpha1.SchemeGroupVersion.WithKind(command.CollectionKind(r.Kind)))
	if err != nil {
		return err
	}

	// Watch changes to primary command resources
	blder := ctrl.NewControllerManagedBy(mgr).For(o)

	// Watch for changes to secondary resource Pods and requeue the owner command resource
	blder = blder.Owns(&corev1.Pod{})

	// Watch for changes to resource Workspace and requeue the associated commands
	blder.Watches(&source.Kind{Type: &v1alpha1.Workspace{}}, &handler.EnqueueRequestsFromMapFunc{
		ToRequests: handler.ToRequestsFunc(func(a handler.MapObject) []reconcile.Request {
			err = r.List(context.TODO(), oList, client.InNamespace(a.Meta.GetNamespace()), client.MatchingLabels{
				"workspace": a.Meta.GetName(),
			})
			if err != nil {
				return []reconcile.Request{}
			}

			rr := []reconcile.Request{}
			meta.EachListItem(oList, func(o runtime.Object) error {
				cmd := o.(command.Interface)
				rr = append(rr, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      cmd.GetName(),
						Namespace: cmd.GetNamespace(),
					},
				})
				return nil
			})
			return rr
		}),
	})

	return blder.Complete(r)
}
