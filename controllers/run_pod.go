package controllers

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1alpha1 "github.com/leg100/stok/api/stok.goalspike.com/v1alpha1"
	"github.com/leg100/stok/pkg/runner"
)

func (r *RunReconciler) reconcilePod(request reconcile.Request, run *v1alpha1.Run, ws *v1alpha1.Workspace) (reconcile.Result, error) {
	// Check if pod exists already
	pod := &corev1.Pod{}
	err := r.Get(context.TODO(), request.NamespacedName, pod)
	if errors.IsNotFound(err) {
		return r.create(run, ws)
	}
	if err != nil {
		return reconcile.Result{}, err
	}

	return r.updateStatus(pod, run, ws)
}

func (r *RunReconciler) updateStatus(pod *corev1.Pod, run *v1alpha1.Run, ws *v1alpha1.Workspace) (reconcile.Result, error) {
	// Signal pod completion to workspace
	switch pod.Status.Phase {
	case corev1.PodSucceeded, corev1.PodFailed:
		run.SetPhase(v1alpha1.RunPhaseCompleted)
	case corev1.PodRunning:
		run.SetPhase(v1alpha1.RunPhaseRunning)
	case corev1.PodPending:
		return reconcile.Result{Requeue: true}, nil
	case corev1.PodUnknown:
		return reconcile.Result{}, fmt.Errorf("State of pod could not be obtained")
	default:
		return reconcile.Result{}, fmt.Errorf("Unknown pod phase: %s", pod.Status.Phase)
	}

	err := r.Status().Update(context.TODO(), run)
	return reconcile.Result{}, err
}

// Create pod
func (r RunReconciler) create(run *v1alpha1.Run, ws *v1alpha1.Workspace) (reconcile.Result, error) {
	pod := runner.NewRunPod(run, ws, r.Image)

	// Set Run instance as the owner and controller
	if err := controllerutil.SetControllerReference(run, pod, r.Scheme); err != nil {
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

	run.SetPhase(v1alpha1.RunPhaseProvisioning)
	if err := r.Status().Update(context.TODO(), run); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{Requeue: true}, nil
}
