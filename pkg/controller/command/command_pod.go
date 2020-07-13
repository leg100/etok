package command

import (
	"context"
	"fmt"
	"path/filepath"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/leg100/stok/constants"
	v1alpha1 "github.com/leg100/stok/pkg/apis/stok/v1alpha1"
	"github.com/leg100/stok/pkg/apis/stok/v1alpha1/command"
	"github.com/leg100/stok/version"
	operatorstatus "github.com/operator-framework/operator-sdk/pkg/status"
)

type CommandPod struct {
	*CommandReconciler
	cmd       command.Interface
	workspace *v1alpha1.Workspace
}

func (cp *CommandPod) secretName() string {
	// TODO: add method to workspace type
	return cp.workspace.Spec.SecretName
}

func (cp *CommandPod) serviceAccountName() string {
	// TODO: add method to workspace type
	return cp.workspace.Spec.ServiceAccountName
}

func (cp *CommandPod) pvcName() string {
	// TODO: add method to workspace type
	return cp.workspace.GetName()
}

func (cp *CommandPod) configMap() string {
	// TODO: add method to command type
	return cp.cmd.GetName()
}

func (cp *CommandPod) reconcile(request reconcile.Request) (reconcile.Result, error) {
	// Check if pod exists already
	pod := &corev1.Pod{}
	err := cp.client.Get(context.TODO(), request.NamespacedName, pod)
	if errors.IsNotFound(err) {
		return cp.create(pod)
	}
	if err != nil {
		return reconcile.Result{}, err
	}

	return cp.updateStatus(pod)
}

func (cp *CommandPod) updateStatus(pod *corev1.Pod) (reconcile.Result, error) {
	// Signal pod completion to workspace
	switch pod.Status.Phase {
	case corev1.PodSucceeded, corev1.PodFailed:
		cp.cmd.GetConditions().SetCondition(operatorstatus.Condition{
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
					cp.cmd.GetConditions().SetCondition(operatorstatus.Condition{
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

	err := cp.client.Status().Update(context.TODO(), cp.cmd)
	return reconcile.Result{}, err
}

// Create pod
func (r CommandPod) create(pod *corev1.Pod) (reconcile.Result, error) {
	// TODO: move to a global variable or dedicated package
	labels := map[string]string{
		"app":       "stok",
		"command":   r.cmd.GetName(),
		"workspace": r.workspace.Name,
	}
	// Use same name as command
	pod.SetName(r.cmd.GetName())
	pod.SetNamespace(r.cmd.GetNamespace())
	pod.SetLabels(labels)

	if err := r.construct(pod); err != nil {
		return reconcile.Result{}, err
	}
	// Set Command instance as the owner and controller
	if err := controllerutil.SetControllerReference(r.cmd, pod, r.scheme); err != nil {
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

func (cp *CommandPod) construct(pod *corev1.Pod) error {
	args := append([]string{"--"}, cp.crd.Wrapper(cp.cmd.GetArgs())...)

	pod.Spec = corev1.PodSpec{
		ServiceAccountName: cp.serviceAccountName(),
		Containers: []corev1.Container{
			{
				Name:            "runner",
				Image:           constants.ImageRepo + ":" + version.Version,
				ImagePullPolicy: corev1.PullIfNotPresent,
				Command:         []string{"stok", "runner"},
				Args:            args,
				Env: []corev1.EnvVar{
					{
						Name:  "STOK_TARBALL_PATH",
						Value: filepath.Join("/tarball", constants.Tarball),
					},
					{
						Name:  "STOK_TIMEOUT_CLIENT",
						Value: cp.cmd.GetTimeoutClient(),
					},
					{
						Name:  "STOK_KIND",
						Value: cp.cmd.GetObjectKind().GroupVersionKind().Kind,
					},
					{
						Name:  "STOK_NAME",
						Value: cp.cmd.GetName(),
					},
					{
						Name:  "STOK_NAMESPACE",
						Value: cp.cmd.GetNamespace(),
					},
				},
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
						MountPath: filepath.Join("/tarball", constants.Tarball),
						SubPath:   constants.Tarball,
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
						ClaimName: cp.pvcName(),
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
							Name: cp.configMap(),
						},
					},
				},
			},
		},
		RestartPolicy: corev1.RestartPolicyNever,
	}

	if cp.secretName() != "" {
		cp.mountCredentials(pod)
	}

	return nil
}

// Mount secret into a volume and set GOOGLE_APPLICATION_CREDENTIALS to
// the hardcoded google credentials file (whether it exists or not). Also
// expose the secret data via environment variables.
func (cp *CommandPod) mountCredentials(pod *corev1.Pod) {
	pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{
		Name: "credentials",
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: cp.secretName(),
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
					Name: cp.secretName(),
				},
			},
		})
}
