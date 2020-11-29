package runner

import (
	"fmt"
	"strings"

	"github.com/leg100/stok/api/stok.goalspike.com/v1alpha1"
	"github.com/leg100/stok/pkg/globals"
	"github.com/leg100/stok/pkg/labels"
	corev1 "k8s.io/api/core/v1"
)

// workspace implements runner, providing a pod on which to setup a new stok
// workspace, which runs terraform init, optionally downloading a custom version
// of terraform, within an init container, and then it runs a standard container
// that simply idles - expressly for performance reasons: it keeps a persistent
// volume attached to the kubernetes node, which means when a run spins up a pod
// the volume can be mounted more quickly (that does mean however that a run pod
// can only be scheduled to the same node as the workspace pod...).
type workspace struct {
	*v1alpha1.Workspace
}

func NewWorkspaceRunner(ws *v1alpha1.Workspace) *workspace {
	return &workspace{ws}
}

func (ws *workspace) Pod(image string) *corev1.Pod {
	pod := Pod(ws, ws.Workspace, image)

	// Permit filtering stok resources by component
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
	var cmds []string
	if ws.Spec.TerraformVersion != "" {
		cmds = ws.terraformDownloadScript()
	}
	cmds = append(cmds, ws.terraformInitScript()...)

	return []string{"sh", "-c", strings.Join(cmds, " && \\\n")}
}

func (ws *workspace) terraformInitScript() []string {
	return []string{
		fmt.Sprintf("terraform init -backend-config=%s", v1alpha1.BackendConfigFilename),
		fmt.Sprintf("terraform workspace select %s-%s || terraform workspace new %[1]s-%[1]s", ws.Namespace, ws.Name),
	}
}

func (ws *workspace) terraformDownloadScript() []string {
	return []string{
		fmt.Sprintf("curl -LOs https://releases.hashicorp.com/terraform/%s/terraform_%[1]s_linux_amd64.zip", ws.Spec.TerraformVersion),
		fmt.Sprintf("curl -LOs https://releases.hashicorp.com/terraform/%s/terraform_%[1]s_SHA256SUMS", ws.Spec.TerraformVersion),
		fmt.Sprintf("sed -n \"/terraform_%s_linux_amd64.zip/p\" terraform_%[1]s_SHA256SUMS | sha256sum -c", ws.Spec.TerraformVersion),
		fmt.Sprintf("mkdir -p %s", globals.TerraformPath),
		fmt.Sprintf("unzip terraform_%s_linux_amd64.zip -d %s", ws.Spec.TerraformVersion, globals.TerraformPath),
		fmt.Sprintf("rm terraform_%s_linux_amd64.zip", ws.Spec.TerraformVersion),
		fmt.Sprintf("rm terraform_%s_SHA256SUMS", ws.Spec.TerraformVersion),
	}
}
