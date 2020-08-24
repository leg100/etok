package controllers

import (
	"path/filepath"

	v1alpha1 "github.com/leg100/stok/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type PodBuilder struct {
	pod      *corev1.Pod
	runner   corev1.Container
	volumes  []corev1.Volume
	mounts   []corev1.VolumeMount
	envs     []corev1.EnvVar
	envFroms []corev1.EnvFromSource
	image    string
}

func NewPodBuilder(namespace, name, image string) *PodBuilder {
	fsgroup := new(int64)
	*fsgroup = 2000

	pod := &corev1.Pod{
		// Need TypeMeta in order to extract Kind later on
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: corev1.PodSpec{
			SecurityContext: &corev1.PodSecurityContext{
				FSGroup: fsgroup,
			},
			RestartPolicy: corev1.RestartPolicyNever,
		},
	}
	return &PodBuilder{pod: pod, image: image}
}

func (pb *PodBuilder) EnableDebug(debug bool) *PodBuilder {
	if debug {
		pb.runner.Args = append([]string{"--debug"}, pb.runner.Args...)
	}
	return pb
}

func (pb *PodBuilder) SetLabels(workspace string) *PodBuilder {
	pb.pod.SetLabels(map[string]string{
		"app":       "stok",
		"workspace": workspace,
	})
	return pb
}

func (pb *PodBuilder) WaitForClient(kind, name, namespace, timeout string) *PodBuilder {
	args := []string{
		"--kind", kind,
		"--name", name,
		"--namespace", namespace,
		"--timeout", timeout,
	}
	pb.runner.Args = append(args, pb.runner.Args...)
	return pb
}

// Finalize building of pod. `init` toggles whether the runner is an init or 'normal' container. If
// true, then it is run as an init container, followed by a normal container that simply idles i.e.
// sleeps for infinity, and restarts upon error. This is for the purpose of running the workspace pod.
func (pb *PodBuilder) Build(init bool) *corev1.Pod {
	pb.pod.Spec.Volumes = pb.volumes
	pb.runner.VolumeMounts = pb.mounts

	pb.runner.Env = pb.envs
	pb.runner.EnvFrom = pb.envFroms

	if init {
		pb.pod.Spec.InitContainers = []corev1.Container{pb.runner}
		pb.pod.Spec.Containers = []corev1.Container{
			{
				Name:                     "idler",
				Image:                    pb.image,
				ImagePullPolicy:          corev1.PullIfNotPresent,
				Command:                  []string{"sh", "-c", "trap \"exit 0\" SIGTERM; while true; do sleep 1; done"},
				TerminationMessagePolicy: "FallbackToLogsOnError",
			},
		}
	} else {
		pb.pod.Spec.Containers = []corev1.Container{pb.runner}
	}

	return pb.pod
}

func (pb *PodBuilder) AddRunnerContainer(args []string) *PodBuilder {
	pb.runner = corev1.Container{
		Name:                     "runner",
		Image:                    pb.image,
		ImagePullPolicy:          corev1.PullIfNotPresent,
		Command:                  []string{"stok", "runner"},
		Args:                     append([]string{"--"}, args...),
		Stdin:                    true,
		TTY:                      true,
		TerminationMessagePolicy: "FallbackToLogsOnError",
	}

	return pb
}

// Mount secret into a volume and set GOOGLE_APPLICATION_CREDENTIALS to
// the hardcoded google credentials file (whether it exists or not). Also
// expose the secret data via environment variables.
func (pb *PodBuilder) AddCredentials(secretname string) *PodBuilder {
	if secretname != "" {
		pb.volumes = append(pb.volumes, corev1.Volume{
			Name: "credentials",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: secretname,
				},
			},
		})

		pb.mounts = append(pb.mounts, corev1.VolumeMount{
			Name:      "credentials",
			MountPath: "/credentials",
		})

		//TODO: we set this regardless of whether google credentials exist and that
		//doesn't cause any obvious problems but really should only set it if they exist
		pb.envs = append(pb.envs, corev1.EnvVar{
			Name:  "GOOGLE_APPLICATION_CREDENTIALS",
			Value: "/credentials/google-credentials.json",
		})

		pb.envFroms = append(pb.envFroms, corev1.EnvFromSource{
			SecretRef: &corev1.SecretEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: secretname,
				},
			},
		})
	}

	return pb
}

func (pb *PodBuilder) AddWorkspace() *PodBuilder {
	pb.volumes = append(pb.volumes, corev1.Volume{
		Name: "workspace",
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	})

	pb.mounts = append(pb.mounts, corev1.VolumeMount{
		Name:      "workspace",
		MountPath: "/workspace",
	})

	pb.runner.WorkingDir = "/workspace"

	return pb
}

func (pb *PodBuilder) AddCache(pvcname string) *PodBuilder {
	pb.volumes = append(pb.volumes, corev1.Volume{
		Name: "cache",
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: pvcname,
			},
		},
	})

	pb.mounts = append(pb.mounts, corev1.VolumeMount{
		Name:      "cache",
		MountPath: "/workspace/.terraform",
	})

	return pb
}

func (pb *PodBuilder) AddBackendConfig(workspacename string) *PodBuilder {
	pb.volumes = append(pb.volumes, corev1.Volume{
		Name: "backendconfig",
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: v1alpha1.BackendConfigMapName(workspacename),
				},
			},
		},
	})

	pb.mounts = append(pb.mounts,
		corev1.VolumeMount{
			Name:      "backendconfig",
			MountPath: "/workspace/" + v1alpha1.BackendTypeFilename,
			SubPath:   v1alpha1.BackendTypeFilename,
			ReadOnly:  true,
		},
		corev1.VolumeMount{
			Name:      "backendconfig",
			MountPath: "/workspace/" + v1alpha1.BackendConfigFilename,
			SubPath:   v1alpha1.BackendConfigFilename,
			ReadOnly:  true,
		},
	)
	return pb
}

func (pb *PodBuilder) MountTarball(configmapname, configmapkey string) *PodBuilder {
	args := []string{
		"--tarball", filepath.Join("/tarball", configmapkey),
		"--path", ".",
	}
	// Prepend rather than append args because they must precede "--"
	pb.runner.Args = append(args, pb.runner.Args...)

	pb.volumes = append(pb.volumes, corev1.Volume{
		Name: "tarball",
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: configmapname,
				},
			},
		},
	})

	pb.mounts = append(pb.mounts, corev1.VolumeMount{
		Name:      "tarball",
		MountPath: filepath.Join("/tarball", configmapkey),
		SubPath:   configmapkey,
	})
	return pb
}

func (pb *PodBuilder) HasServiceAccount(serviceaccountname string) *PodBuilder {
	if serviceaccountname != "" {
		pb.pod.Spec.ServiceAccountName = serviceaccountname
	}
	return pb
}
