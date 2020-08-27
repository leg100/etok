package controllers

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/leg100/stok/api/command"
	v1alpha1 "github.com/leg100/stok/api/v1alpha1"
	operatorstatus "github.com/operator-framework/operator-sdk/pkg/status"
)

type podOpts struct {
	cmd                command.Interface
	workspaceName      string
	secretName         string
	serviceAccountName string
	pvcName            string
	configMapName      string
	configMapKey       string
}

func (r *CommandReconciler) reconcilePod(request reconcile.Request, opts *podOpts) (reconcile.Result, error) {
	// Check if pod exists already
	pod := &corev1.Pod{}
	err := r.Get(context.TODO(), request.NamespacedName, pod)
	if errors.IsNotFound(err) {
		return r.create(opts)
	}
	if err != nil {
		return reconcile.Result{}, err
	}

	return r.updateStatus(pod, opts)
}

// IsSynchronising indicates whether obj is in process of synchronisation between client and pod, or
// not.
func IsSynchronising(obj metav1.Object) bool {
	_, ok := obj.GetAnnotations()[v1alpha1.WaitAnnotationKey]
	return ok
}

func (r *CommandReconciler) updateStatus(pod *corev1.Pod, opts *podOpts) (reconcile.Result, error) {
	// Signal pod completion to workspace
	switch pod.Status.Phase {
	case corev1.PodSucceeded, corev1.PodFailed:
		opts.cmd.SetPhase(v1alpha1.CommandPhaseCompleted)
		opts.cmd.GetConditions().SetCondition(operatorstatus.Condition{
			Type:    v1alpha1.ConditionCompleted,
			Status:  corev1.ConditionTrue,
			Reason:  v1alpha1.ReasonPodCompleted,
			Message: fmt.Sprintf("Pod completed with phase %s", pod.Status.Phase),
		})
	case corev1.PodRunning:
		if IsSynchronising(opts.cmd) {
			opts.cmd.SetPhase(v1alpha1.CommandPhaseSync)
		} else {
			opts.cmd.SetPhase(v1alpha1.CommandPhaseRunning)
		}
	case corev1.PodPending:
		return reconcile.Result{Requeue: true}, nil
	case corev1.PodUnknown:
		return reconcile.Result{}, fmt.Errorf("State of pod could not be obtained")
	default:
		return reconcile.Result{}, fmt.Errorf("Unknown pod phase: %s", pod.Status.Phase)
	}

	err := r.Status().Update(context.TODO(), opts.cmd)
	return reconcile.Result{}, err
}

// Create pod
func (r CommandReconciler) create(opts *podOpts) (reconcile.Result, error) {
	pod := NewPodBuilder(opts.cmd.GetNamespace(), opts.cmd.GetName(), r.Image).
		SetLabels(opts.cmd.GetName(), opts.workspaceName, command.CommandKindToCLI(r.Kind), "runner").
		AddRunnerContainer(opts.cmd.GetArgs()).
		AddWorkspace().
		AddCache(opts.pvcName).
		AddBackendConfig(opts.workspaceName).
		AddCredentials(opts.secretName).
		HasServiceAccount(opts.serviceAccountName).
		MountTarball(opts.configMapName, opts.configMapKey).
		WaitForClient(r.Kind, opts.cmd.GetName(), opts.cmd.GetNamespace(), opts.cmd.GetTimeoutClient()).
		EnableDebug(opts.cmd.GetDebug()).
		Build(false)

	// Set Command instance as the owner and controller
	if err := controllerutil.SetControllerReference(opts.cmd, pod, r.Scheme); err != nil {
		return reconcile.Result{}, err
	}

	err := r.Create(context.TODO(), pod)
	// ignore error wherein two reconciles in quick succession try to create obj
	if errors.IsAlreadyExists(err) {
		return reconcile.Result{}, nil
	}
	if err != nil {
		return reconcile.Result{}, err
	}

	opts.cmd.SetPhase(v1alpha1.CommandPhaseProvisioning)
	if err := r.Status().Update(context.TODO(), opts.cmd); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{Requeue: true}, nil
}
