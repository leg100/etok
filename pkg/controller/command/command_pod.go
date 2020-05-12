package command

import (
	"path/filepath"

	corev1 "k8s.io/api/core/v1"

	"github.com/leg100/stok/constants"
	"github.com/leg100/stok/crdinfo"
	"k8s.io/apimachinery/pkg/runtime"
)

type CommandPod struct {
	corev1.Pod
	resource           string
	configMap          string
	serviceAccountName string
	secretName         string
	workspaceName      string
	pvcName            string
	args               []string
	crd                crdinfo.CRDInfo
}

func (cp *CommandPod) GetRuntimeObj() runtime.Object {
	return &cp.Pod
}

func (cp *CommandPod) Construct() error {
	script, err := Script{
		Resource:   cp.resource,
		Tarball:    constants.Tarball,
		Entrypoint: cp.crd.Entrypoint,
		Kind:       cp.crd.APISingular,
		Args:       cp.args,
	}.generate()
	if err != nil {
		return err
	}

	cp.Spec = corev1.PodSpec{
		ServiceAccountName: cp.serviceAccountName,
		Containers: []corev1.Container{
			{
				Name:            "terraform",
				Image:           "leg100/terraform:latest",
				ImagePullPolicy: corev1.PullIfNotPresent,
				Command:         []string{"sh"},
				Args:            []string{"-o", "pipefail", "-ec", script},
				Stdin:           true,
				Env: []corev1.EnvVar{
					{
						Name:  "GOOGLE_APPLICATION_CREDENTIALS",
						Value: "/credentials/google-credentials.json",
					},
				},
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
						Name:      "credentials",
						MountPath: "/credentials",
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
						ClaimName: cp.pvcName,
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
							Name: cp.configMap,
						},
					},
				},
			},
			{
				Name: "credentials",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: cp.secretName,
					},
				},
			},
		},
		RestartPolicy: corev1.RestartPolicyNever,
	}
	return nil
}
