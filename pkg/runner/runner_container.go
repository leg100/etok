package runner

import (
	"fmt"
	"path/filepath"
	"strconv"

	corev1 "k8s.io/api/core/v1"

	"github.com/leg100/etok/api/etok.dev/v1alpha1"
	"github.com/leg100/etok/pkg/globals"
)

func Container(r Runner, ws *v1alpha1.Workspace, image string) corev1.Container {
	container := corev1.Container{
		Env: []corev1.EnvVar{
			{
				Name:  "ETOK_HANDSHAKE",
				Value: strconv.FormatBool(r.GetHandshake()),
			},
			{
				Name:  "ETOK_HANDSHAKE_TIMEOUT",
				Value: r.GetHandshakeTimeout(),
			},
		},
		Name:                     globals.RunnerContainerName,
		Image:                    image,
		ImagePullPolicy:          corev1.PullIfNotPresent,
		Command:                  []string{"etok", "runner"},
		Stdin:                    true,
		TTY:                      true,
		TerminationMessagePolicy: "FallbackToLogsOnError",
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      backendConfigVolumeName,
				MountPath: filepath.Join(r.WorkingDir(), v1alpha1.BackendTypeFilename),
				SubPath:   v1alpha1.BackendTypeFilename,
				ReadOnly:  true,
			},
			{
				Name:      backendConfigVolumeName,
				MountPath: filepath.Join(r.WorkingDir(), v1alpha1.BackendConfigFilename),
				SubPath:   v1alpha1.BackendConfigFilename,
				ReadOnly:  true,
			},
			{
				Name:      cacheVolumeName,
				MountPath: filepath.Join(r.WorkingDir(), terraformDotPath),
				SubPath:   terraformDotPath,
			},
		},
		WorkingDir: r.WorkingDir(),
	}

	var args []string
	if r.GetVerbosity() > 0 {
		// Set non-defaut verbose logging for the runner process
		args = append(args, fmt.Sprintf("-v=%d", r.GetVerbosity()))
	}

	// The runner process expects args to come after --
	args = append(args, "--")

	container.Args = append(args, r.ContainerArgs()...)

	if ws.Spec.Backend.Type == "" || ws.Spec.Backend.Type == "local" {
		container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
			Name:      cacheVolumeName,
			MountPath: filepath.Join(r.WorkingDir(), terraformLocalStatePath),
			SubPath:   terraformLocalStatePath,
		})
	}

	if ws.Spec.TerraformVersion != "" {
		// Custom terraform version specified; mount a directory from the cache
		// volume so that when it is downloaded it is cached
		container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
			Name: cacheVolumeName,
			// Path in container to mount vol at
			MountPath: terraformBinMountPath,
			// Path within PVC to mount
			SubPath: terraformBinSubPath,
		})
	}

	if ws.Spec.SecretName != "" {
		container.EnvFrom = append(container.EnvFrom, corev1.EnvFromSource{
			SecretRef: &corev1.SecretEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: ws.Spec.SecretName,
				},
			},
		})
	}

	return container
}
