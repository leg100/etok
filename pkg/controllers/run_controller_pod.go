package controllers

import (
	"path/filepath"
	"strconv"

	"github.com/leg100/etok/api/etok.dev/v1alpha1"
	"github.com/leg100/etok/pkg/globals"
	"github.com/leg100/etok/pkg/labels"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func runPod(run *v1alpha1.Run, ws *v1alpha1.Workspace, image string) *corev1.Pod {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      run.PodName(),
			Namespace: run.Namespace,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Command: []string{"etok", "runner"},
					Args:    append([]string{"--"}, run.Args...),
					Env: []corev1.EnvVar{
						{
							Name:  "ETOK_HANDSHAKE",
							Value: strconv.FormatBool(run.Handshake),
						},
						{
							Name:  "ETOK_HANDSHAKE_TIMEOUT",
							Value: run.HandshakeTimeout,
						},
						{
							Name:  "ETOK_COMMAND",
							Value: run.Command,
						},
						{
							Name:  "ETOK_WORKSPACE",
							Value: ws.TerraformName(),
						},
						{
							Name:  "ETOK_DEST",
							Value: workspaceDir,
						},
						{
							Name:  "ETOK_TARBALL",
							Value: filepath.Join("/tarball", run.ConfigMapKey),
						},
						{
							Name:  "ETOK_V",
							Value: strconv.Itoa(run.Verbosity),
						},
						{
							Name:  "TF_PLUGIN_CACHE_DIR",
							Value: pluginMountPath,
						},
					},
					Image:                    image,
					ImagePullPolicy:          corev1.PullIfNotPresent,
					Name:                     globals.RunnerContainerName,
					Stdin:                    run.Handshake,
					TTY:                      run.Handshake,
					TerminationMessagePolicy: "FallbackToLogsOnError",
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "cache",
							MountPath: pluginMountPath,
							SubPath:   pluginSubPath,
						},
						{
							Name:      "cache",
							MountPath: binMountPath,
							SubPath:   binSubPath,
						},
						{
							Name:      "tarball",
							MountPath: filepath.Join("/tarball", run.ConfigMapKey),
							SubPath:   run.ConfigMapKey,
						},
					},
					WorkingDir: filepath.Join(workspaceDir, run.ConfigMapPath),
				},
			},
			RestartPolicy:      corev1.RestartPolicyNever,
			ServiceAccountName: ws.Spec.ServiceAccountName,
			Volumes: []corev1.Volume{
				{
					Name: "cache",
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: ws.PVCName(),
						},
					},
				},
				{
					Name: "tarball",
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: run.ConfigMap,
							},
						},
					},
				},
			},
		},
	}

	// Set etok's common labels
	labels.SetCommonLabels(pod)
	// Permit filtering pods by workspace
	labels.SetLabel(pod, labels.Workspace(ws.Name))
	// Permit filtering etok resources by component
	labels.SetLabel(pod, labels.RunComponent)
	// Permit filtering pods by the run command
	labels.SetLabel(pod, labels.Command(run.Command))

	if ws.Spec.SecretName != "" {
		pod.Spec.Containers[0].EnvFrom = append(pod.Spec.Containers[0].EnvFrom, corev1.EnvFromSource{
			SecretRef: &corev1.SecretEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: ws.Spec.SecretName,
				},
			},
		})
	}

	return pod
}
