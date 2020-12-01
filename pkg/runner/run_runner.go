package runner

import (
	"path/filepath"
	"strings"

	"github.com/leg100/etok/api/etok.dev/v1alpha1"
	"github.com/leg100/etok/pkg/labels"
	corev1 "k8s.io/api/core/v1"
)

// run implements runner, providing a pod on which a run's command is executed
// (typically terraform).
type run struct {
	*v1alpha1.Run
}

func NewRunPod(schema *v1alpha1.Run, ws *v1alpha1.Workspace, image string) *corev1.Pod {
	r := &run{schema}
	pod := Pod(r, ws, image)

	// Permit filtering etok resources by component
	labels.SetLabel(pod, labels.RunComponent)

	// Permit filtering pods by the run command
	labels.SetLabel(pod, labels.Command(r.Command))

	container := Container(r, ws, image)
	container.Env = append(container.Env, corev1.EnvVar{
		Name:  "TF_WORKSPACE",
		Value: ws.TerraformName(),
	})
	container.Env = append(container.Env, corev1.EnvVar{
		Name:  "ETOK_PATH",
		Value: ".",
	})
	container.Env = append(container.Env, corev1.EnvVar{
		Name:  "ETOK_TARBALL",
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

func (r *run) WorkingDir() string {
	return filepath.Join("/workspace", r.ConfigMapPath)
}

func (r *run) ContainerArgs() (args []string) {
	if r.Command != "sh" {
		// Any command other than sh is a terraform command
		args = append(args, "terraform")
	}

	args = append(args, strings.Split(r.Command, " ")...)
	args = append(args, r.Args...)

	return args
}
