package controllers

import (
	"bytes"

	"github.com/leg100/etok/api/etok.dev/v1alpha1"
	"github.com/leg100/etok/pkg/labels"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	InstallerContainerName = "installer"
	idlerCommand           = "trap \"exit 0\" SIGTERM; while true; do sleep 1; done"
)

// WorkspacePod returns a pod on which to setup a new etok workspace, optionally
// downloading a custom version of terraform, within an init container, and then
// it runs a standard container that simply idles - expressly for performance
// reasons: it keeps a persistent volume attached to the kubernetes node, which
// means when a run spins up a pod the volume can be mounted more quickly (that
// does mean however that a run pod can only be scheduled to the same node as
// the workspace pod...).
func WorkspacePod(ws *v1alpha1.Workspace, image string) (*corev1.Pod, error) {
	script := new(bytes.Buffer)
	if err := generateScript(script, ws); err != nil {
		return nil, err
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ws.PodName(),
			Namespace: ws.Namespace,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:                     "idler",
					Image:                    image,
					ImagePullPolicy:          corev1.PullIfNotPresent,
					Command:                  []string{"sh", "-c", idlerCommand},
					TerminationMessagePolicy: "FallbackToLogsOnError",
				},
			},
			InitContainers: []corev1.Container{
				{
					Name:                     InstallerContainerName,
					Image:                    image,
					ImagePullPolicy:          corev1.PullIfNotPresent,
					Command:                  []string{"sh", "-c", script.String()},
					TerminationMessagePolicy: "FallbackToLogsOnError",
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "cache",
							MountPath: binMountPath,
							SubPath:   binSubPath,
						},
					},
				},
			},
			RestartPolicy:      corev1.RestartPolicyAlways,
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
			},
		},
	}

	// Set etok's common labels
	labels.SetCommonLabels(pod)
	// Permit filtering pods by workspace
	labels.SetLabel(pod, labels.Workspace(ws.Name))
	// Permit filtering resources by component
	labels.SetLabel(pod, labels.WorkspaceComponent)

	return pod, nil
}
