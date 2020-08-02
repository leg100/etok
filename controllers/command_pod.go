package controllers

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/leg100/stok/api/command"
	v1alpha1 "github.com/leg100/stok/api/v1alpha1"
	"github.com/leg100/stok/version"
	operatorstatus "github.com/operator-framework/operator-sdk/pkg/status"
)

type podOpts struct {
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

	return r.updateStatus(pod)
}

func (r *CommandReconciler) updateStatus(pod *corev1.Pod) (reconcile.Result, error) {
	// Signal pod completion to workspace
	switch pod.Status.Phase {
	case corev1.PodSucceeded, corev1.PodFailed:
		r.C.GetConditions().SetCondition(operatorstatus.Condition{
			Type:    v1alpha1.ConditionCompleted,
			Status:  corev1.ConditionTrue,
			Reason:  v1alpha1.ReasonPodCompleted,
			Message: fmt.Sprintf("Pod completed with phase %s", pod.Status.Phase),
		})
	case corev1.PodRunning:
		conditions := pod.Status.Conditions
		if conditions != nil {
			for i := range conditions {
				if conditions[i].Type == corev1.PodReady &&
					conditions[i].Status == corev1.ConditionTrue {
					r.C.GetConditions().SetCondition(operatorstatus.Condition{
						Type:    v1alpha1.ConditionAttachable,
						Status:  corev1.ConditionTrue,
						Reason:  v1alpha1.ReasonPodRunningAndReady,
						Message: "Pod is running and ready and attachable",
					})
				}
			}
		}
	case corev1.PodPending:
		// TODO: not sure if requeue is necessary:
		// https://github.com/operator-framework/operator-sdk/issues/2898#issuecomment-623883813
		return reconcile.Result{Requeue: true}, nil
	case corev1.PodUnknown:
		return reconcile.Result{}, fmt.Errorf("State of pod could not be obtained")
	default:
		return reconcile.Result{}, fmt.Errorf("Unknown pod phase: %s", pod.Status.Phase)
	}

	err := r.Status().Update(context.TODO(), r.C)
	return reconcile.Result{}, err
}

// Create pod
func (r CommandReconciler) create(opts *podOpts) (reconcile.Result, error) {
	pod := NewPodBuilder(r.C.GetNamespace(), r.C.GetName(), r.RunnerImage).
		AddRunnerContainer(r.C.GetArgs()).
		AddWorkspace().
		AddCache(opts.pvcName).
		AddBackendConfig(opts.workspaceName).
		AddCredentials(opts.secretName).
		HasServiceAccount(opts.serviceAccountName).
		MountTarball(opts.configMapName, opts.configMapKey).
		WaitForClient(r.Kind, r.C.GetName(), r.C.GetNamespace(), r.C.GetTimeoutClient()).
		EnableDebug(r.C.GetDebug()).
		Build(false)

	// TODO: move to a global variable or dedicated package
	pod.SetLabels(map[string]string{
		"app":       "stok",
		"command":   command.CommandKindToCLI(r.C.GroupVersionKind().Kind),
		"workspace": opts.workspaceName,
		"version":   version.Version,
	})

	// Set Command instance as the owner and controller
	if err := controllerutil.SetControllerReference(r.C, pod, r.Scheme); err != nil {
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

	return reconcile.Result{Requeue: true}, nil
}
