package command

import (
	"context"
	"fmt"
	"path/filepath"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1alpha1 "github.com/leg100/stok/pkg/apis/stok/v1alpha1"
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
	err := r.client.Get(context.TODO(), request.NamespacedName, pod)
	if errors.IsNotFound(err) {
		return r.create(pod, opts)
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
		r.c.GetConditions().SetCondition(operatorstatus.Condition{
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
					r.c.GetConditions().SetCondition(operatorstatus.Condition{
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

	err := r.client.Status().Update(context.TODO(), r.c)
	return reconcile.Result{}, err
}

// Create pod
func (r CommandReconciler) create(pod *corev1.Pod, opts *podOpts) (reconcile.Result, error) {
	// TODO: move to a global variable or dedicated package
	labels := map[string]string{
		"app":       "stok",
		"command":   v1alpha1.CommandKindToCLI(r.c.GroupVersionKind().Kind),
		"workspace": opts.workspaceName,
		"version":   version.Version,
	}
	// Use same name as command
	pod.SetName(r.c.GetName())
	pod.SetNamespace(r.c.GetNamespace())
	pod.SetLabels(labels)

	if err := r.construct(pod, opts); err != nil {
		return reconcile.Result{}, err
	}
	// Set Command instance as the owner and controller
	if err := controllerutil.SetControllerReference(r.c, pod, r.scheme); err != nil {
		return reconcile.Result{}, err
	}

	err := r.client.Create(context.TODO(), pod)
	// ignore error wherein two reconciles in quick succession try to create obj
	if errors.IsAlreadyExists(err) {
		return reconcile.Result{}, nil
	}
	if err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{Requeue: true}, nil
}

func (r *CommandReconciler) runnerArgs(opts *podOpts) []string {
	var args []string
	if r.c.GetDebug() {
		args = append(args, "--debug")
	}
	args = append(args, "--tarball", filepath.Join("/tarball", opts.configMapKey))
	args = append(args, "--timeout", r.c.GetTimeoutClient())
	args = append(args, "--path", ".")
	args = append(args, "--kind", r.c.GetObjectKind().GroupVersionKind().Kind)
	args = append(args, "--name", r.c.GetName())
	args = append(args, "--namespace", r.c.GetNamespace())
	args = append(args, "--")
	args = append(args, r.c.GetArgs()...)
	return args
}

func (r *CommandReconciler) construct(pod *corev1.Pod, opts *podOpts) error {
	pod.Spec = corev1.PodSpec{
		Containers: []corev1.Container{
			{
				Name:                     "runner",
				Image:                    "leg100/stok:" + version.Version,
				ImagePullPolicy:          corev1.PullIfNotPresent,
				Command:                  []string{"stok", "runner"},
				Args:                     r.runnerArgs(opts),
				Stdin:                    true,
				TTY:                      true,
				TerminationMessagePolicy: "FallbackToLogsOnError",
				VolumeMounts: []corev1.VolumeMount{
					{
						Name:      "workspace",
						MountPath: "/workspace",
					},
					{
						Name:      "cache",
						MountPath: "/workspace/.terraform",
					},
					{
						Name:      "tarball",
						MountPath: filepath.Join("/tarball", opts.configMapKey),
						SubPath:   opts.configMapKey,
					},
				},
				WorkingDir: "/workspace",
			},
		},
		Volumes: []corev1.Volume{
			{
				Name: "cache",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: opts.pvcName,
					},
				},
			},
			{
				Name: "workspace",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			},
			{
				Name: "tarball",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: opts.configMapName,
						},
					},
				},
			},
		},
		RestartPolicy: corev1.RestartPolicyNever,
	}

	if opts.serviceAccountName != "" {
		pod.Spec.ServiceAccountName = opts.serviceAccountName
	}

	if opts.secretName != "" {
		r.mountCredentials(pod, opts)
	}

	return nil
}

// Mount secret into a volume and set GOOGLE_APPLICATION_CREDENTIALS to
// the hardcoded google credentials file (whether it exists or not). Also
// expose the secret data via environment variables.
func (r *CommandReconciler) mountCredentials(pod *corev1.Pod, opts *podOpts) {
	pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{
		Name: "credentials",
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: opts.secretName,
			},
		},
	})

	pod.Spec.Containers[0].VolumeMounts = append(pod.Spec.Containers[0].VolumeMounts,
		corev1.VolumeMount{
			Name:      "credentials",
			MountPath: "/credentials",
		})

	//TODO: we set this regardless of whether google credentials exist and that
	//doesn't cause any obvious problems but really should only set it if they exist
	pod.Spec.Containers[0].Env = append(pod.Spec.Containers[0].Env,
		corev1.EnvVar{
			Name:  "GOOGLE_APPLICATION_CREDENTIALS",
			Value: "/credentials/google-credentials.json",
		})

	pod.Spec.Containers[0].EnvFrom = append(pod.Spec.Containers[0].EnvFrom,
		corev1.EnvFromSource{
			SecretRef: &corev1.SecretEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: opts.secretName,
				},
			},
		})
}
