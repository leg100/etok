package controllers

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1alpha1 "github.com/leg100/stok/api/stok.goalspike.com/v1alpha1"
	operatorstatus "github.com/operator-framework/operator-sdk/pkg/status"
)

type podOpts struct {
	run                *v1alpha1.Run
	workspaceName      string
	secretName         string
	serviceAccountName string
	pvcName            string
	configMapName      string
	configMapKey       string
}

func (r *RunReconciler) reconcilePod(request reconcile.Request, opts *podOpts) (reconcile.Result, error) {
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

func (r *RunReconciler) updateStatus(pod *corev1.Pod, opts *podOpts) (reconcile.Result, error) {
	// Signal pod completion to workspace
	switch pod.Status.Phase {
	case corev1.PodSucceeded, corev1.PodFailed:
		opts.run.SetPhase(v1alpha1.RunPhaseCompleted)
		opts.run.GetConditions().SetCondition(operatorstatus.Condition{
			Type:    v1alpha1.ConditionCompleted,
			Status:  corev1.ConditionTrue,
			Reason:  v1alpha1.ReasonPodCompleted,
			Message: fmt.Sprintf("Pod completed with phase %s", pod.Status.Phase),
		})
	case corev1.PodRunning:
		if IsSynchronising(opts.run) {
			opts.run.SetPhase(v1alpha1.RunPhaseSync)
		} else {
			opts.run.SetPhase(v1alpha1.RunPhaseRunning)
		}
	case corev1.PodPending:
		return reconcile.Result{Requeue: true}, nil
	case corev1.PodUnknown:
		return reconcile.Result{}, fmt.Errorf("State of pod could not be obtained")
	default:
		return reconcile.Result{}, fmt.Errorf("Unknown pod phase: %s", pod.Status.Phase)
	}

	err := r.Status().Update(context.TODO(), opts.run)
	return reconcile.Result{}, err
}

func prependTerraformToArgs(run *v1alpha1.Run, args []string) []string {
	if run.Command == "sh" {
		return args
	}
	return append([]string{"terraform"}, args...)
}

//func setDefaultIfExists(rc client.Client, defname string, namespace string, obj runtime.Object) (string, error) {
//	nn := types.NamespacedName{Name: defname, Namespace: namespace}
//	if err := rc.Get(context.TODO(), nn, obj); err != nil {
//		if errors.IsNotFound(err) {
//			return "",
//
//}

// Create pod
func (r RunReconciler) create(opts *podOpts) (reconcile.Result, error) {
	args := append(strings.Split(opts.run.Command, " "), opts.run.GetArgs()...)
	pod := NewPodBuilder(opts.run.GetNamespace(), opts.run.GetName(), r.Image).
		SetLabels(opts.run.GetName(), opts.workspaceName, opts.run.Command, "runner").
		AddRunnerContainer(prependTerraformToArgs(opts.run, args)).
		AddWorkspace().
		AddCache(opts.pvcName).
		AddBackendConfig(opts.workspaceName).
		AddCredentials(opts.secretName).
		HasServiceAccount(opts.serviceAccountName).
		MountTarball(opts.configMapName, opts.configMapKey).
		WaitForClient("Run", opts.run.GetName(), opts.run.GetNamespace(), opts.run.GetTimeoutClient()).
		EnableDebug(opts.run.GetDebug()).
		Build(false)

	// Set Run instance as the owner and controller
	if err := controllerutil.SetControllerReference(opts.run, pod, r.Scheme); err != nil {
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

	opts.run.SetPhase(v1alpha1.RunPhaseProvisioning)
	if err := r.Status().Update(context.TODO(), opts.run); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{Requeue: true}, nil
}
