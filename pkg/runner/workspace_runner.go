package runner

import (
	"bytes"

	"github.com/leg100/etok/api/etok.dev/v1alpha1"
	"github.com/leg100/etok/pkg/labels"
	corev1 "k8s.io/api/core/v1"
)

// workspace implements runner, providing a pod on which to setup a new etok
// workspace, which runs terraform init, optionally downloading a custom version
// of terraform, within an init container, and then it runs a standard container
// that simply idles - expressly for performance reasons: it keeps a persistent
// volume attached to the kubernetes node, which means when a run spins up a pod
// the volume can be mounted more quickly (that does mean however that a run pod
// can only be scheduled to the same node as the workspace pod...).
type workspace struct {
	*v1alpha1.Workspace
}

func NewWorkspacePod(schema *v1alpha1.Workspace, image string) *corev1.Pod {
	ws := &workspace{schema}
	pod := Pod(ws, ws.Workspace, image)

	// Permit filtering etok resources by component
	labels.SetLabel(pod, labels.WorkspaceComponent)

	pod.Spec.InitContainers = []corev1.Container{
		Container(ws, ws.Workspace, image),
	}

	// A container that simply idles i.e.  sleeps for infinity, and restarts upon error.
	pod.Spec.Containers = []corev1.Container{
		{
			Name:                     "idler",
			Image:                    image,
			ImagePullPolicy:          corev1.PullIfNotPresent,
			Command:                  []string{"sh", "-c", "trap \"exit 0\" SIGTERM; while true; do sleep 1; done"},
			TerminationMessagePolicy: "FallbackToLogsOnError",
		},
	}
	return pod
}

func (ws *workspace) GetHandshake() bool          { return ws.Spec.AttachSpec.Handshake }
func (ws *workspace) GetHandshakeTimeout() string { return ws.Spec.AttachSpec.HandshakeTimeout }

func (ws *workspace) GetVerbosity() int { return ws.Spec.Verbosity }

func (ws *workspace) WorkingDir() string { return "/workspace" }

func (ws *workspace) ContainerArgs() []string {
	buf := new(bytes.Buffer)
	if err := generateWorkspaceScript(buf, ws.Workspace); err != nil {
		// TODO: propagate error
		panic(err.Error())
	}
	return []string{"sh", "-c", buf.String()}
}
