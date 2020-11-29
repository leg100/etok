package runner

import (
	"fmt"
	"path/filepath"
	"strconv"

	corev1 "k8s.io/api/core/v1"

	"github.com/leg100/stok/api/stok.goalspike.com/v1alpha1"
	"github.com/leg100/stok/pkg/globals"
)

func Container(r Runner, ws *v1alpha1.Workspace, image string) corev1.Container {
	container := corev1.Container{
		Env: []corev1.EnvVar{
			{
				Name:  "STOK_HANDSHAKE",
				Value: strconv.FormatBool(r.GetHandshake()),
			},
			{
				Name:  "STOK_HANDSHAKE_TIMEOUT",
				Value: r.GetHandshakeTimeout(),
			},
		},
		Name:                     globals.RunnerContainerName,
		Image:                    image,
		ImagePullPolicy:          corev1.PullIfNotPresent,
		Command:                  []string{"stok", "runner"},
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
				MountPath: filepath.Join(r.WorkingDir(), dotTerraformPath),
				SubPath:   dotTerraformPath,
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
			MountPath: filepath.Join(r.WorkingDir(), localTerraformStatePath),
			SubPath:   localTerraformStatePath,
		})
	}

	if ws.Spec.TerraformVersion != "" {
		// Custom terraform version specified; mount a directory from the cache
		// volume so that when it is downloaded it is cached
		container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
			Name: cacheVolumeName,
			// Path in container to mount vol at
			MountPath: globals.TerraformPath,
			// Path within PVC to mount
			SubPath: terraformBinPath,
		})
	}

	if ws.Spec.SecretName != "" {
		// Mount secret into a volume and set GOOGLE_APPLICATION_CREDENTIALS to
		// the hardcoded google credentials file (whether it exists or not). Also
		// expose the secret data via environment variables.
		container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
			Name:      credentialsVolumeName,
			MountPath: "/credentials",
		})

		//TODO: we set this regardless of whether google credentials exist and that
		//doesn't cause any obvious problems but really should only set it if they exist
		container.Env = append(container.Env, corev1.EnvVar{
			Name:  "GOOGLE_APPLICATION_CREDENTIALS",
			Value: "/credentials/google-credentials.json",
		})

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
