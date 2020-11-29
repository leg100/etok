package runner

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/leg100/stok/api/stok.goalspike.com/v1alpha1"
	"github.com/leg100/stok/pkg/labels"
	corev1 "k8s.io/api/core/v1"
)

// run implements runner, providing a pod on which a run's command is executed
// (typically terraform).
type run struct {
	*v1alpha1.Run
	ws *v1alpha1.Workspace
}

func NewRunRunner(r *v1alpha1.Run, ws *v1alpha1.Workspace) *run {
	return &run{r, ws}
}

func (r *run) Pod(image string) *corev1.Pod {
	pod := Pod(r, r.ws, image)

	// Permit filtering stok resources by component
	labels.SetLabel(pod, labels.RunComponent)

	// Permit filtering pods by the run command
	labels.SetLabel(pod, labels.Command(r.Command))

	container := Container(r, r.ws, image)
	container.Env = append(container.Env, corev1.EnvVar{
		Name:  "TF_WORKSPACE",
		Value: fmt.Sprintf("%s-%s", r.ws.Namespace, r.ws.Name),
	})
	container.Env = append(container.Env, corev1.EnvVar{
		Name:  "STOK_PATH",
		Value: ".",
	})
	container.Env = append(container.Env, corev1.EnvVar{
		Name:  "STOK_TARBALL",
		Value: filepath.Join("/tarball", r.ConfigMapKey),
	})
	container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
		Name:      "tarball",
		MountPath: filepath.Join("/tarball", r.ConfigMapKey),
		SubPath:   r.ConfigMapKey,
	})

	pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{
		Name: "tarball",
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: r.ConfigMap,
				},
			},
		},
	})

	pod.Spec.Containers = []corev1.Container{container}
	return pod
}

func (r *run) GetHandshake() bool          { return r.AttachSpec.Handshake }
func (r *run) GetHandshakeTimeout() string { return r.AttachSpec.HandshakeTimeout }

func (r *run) GetVerbosity() int { return r.Verbosity }

func (r *run) WorkingDir() string { return "/runner" }

func (r *run) ContainerArgs() (args []string) {
	if r.Command != "sh" {
		// Any command other than sh is a terraform command
		args = append(args, "terraform")
	}

	args = append(args, strings.Split(r.Command, " ")...)
	args = append(args, r.Args...)

	return args
}
