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
	command            Command
	configMap          string
	serviceAccountName string
	secretName         string
	workspaceName      string
	pvcName            string
	crd                crdinfo.CRDInfo
}

func (cp *CommandPod) GetRuntimeObj() runtime.Object {
	return &cp.Pod
}

func (cp *CommandPod) Construct() error {
	script, err := Script{
		Resource:      cp.command.GetName(),
		Tarball:       constants.Tarball,
		Entrypoint:    cp.crd.Entrypoint,
		Kind:          cp.crd.APIPlural,
		Args:          cp.command.GetArgs(),
		TimeoutClient: cp.command.GetTimeoutClient(),
		TimeoutQueue:  cp.command.GetTimeoutQueue(),
	}.generate()
	if err != nil {
		return err
	}

	cp.Spec = corev1.PodSpec{
		ServiceAccountName: cp.serviceAccountName,
		Containers: []corev1.Container{
			{
				Name:                     "terraform",
				Image:                    "leg100/terraform:latest",
				ImagePullPolicy:          corev1.PullIfNotPresent,
				Command:                  []string{"sh"},
				Args:                     []string{"-o", "pipefail", "-ec", script},
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
		},
		RestartPolicy: corev1.RestartPolicyNever,
	}

	if cp.secretName != "" {
		cp.mountCredentials()
	}

	return nil
}

// Mount secret into a volume and set GOOGLE_APPLICATION_CREDENTIALS to
// the hardcoded google credentials file (whether it exists or not). Also
// expose the secret data via environment variables.
func (cp *CommandPod) mountCredentials() {
	cp.Spec.Volumes = append(cp.Spec.Volumes, corev1.Volume{
		Name: "credentials",
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: cp.secretName,
			},
		},
	})

	cp.Spec.Containers[0].VolumeMounts = append(cp.Spec.Containers[0].VolumeMounts,
		corev1.VolumeMount{
			Name:      "credentials",
			MountPath: "/credentials",
		})

	//TODO: we set this regardless of whether google credentials exist and that
	//doesn't cause any obvious problems but really should only set it if they exist
	cp.Spec.Containers[0].Env = append(cp.Spec.Containers[0].Env,
		corev1.EnvVar{
			Name:  "GOOGLE_APPLICATION_CREDENTIALS",
			Value: "/credentials/google-credentials.json",
		})

	cp.Spec.Containers[0].EnvFrom = append(cp.Spec.Containers[0].EnvFrom,
		corev1.EnvFromSource{
			SecretRef: &corev1.SecretEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: cp.secretName,
				},
			},
		})
}
