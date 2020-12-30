package v1alpha1

import (
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	DefaultHandshakeTimeout = 10 * time.Second
)

func init() {
	SchemeBuilder.Register(&Run{}, &RunList{})
}

// Run is the Schema for the runs API
// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Command",type="string",JSONPath=".spec.command"
// +kubebuilder:printcolumn:name="Workspace",type="string",JSONPath=".spec.workspace"
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"

type Run struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	RunSpec   `json:"spec,omitempty"`
	RunStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// RunList contains a list of Run
type RunList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Run `json:"items"`
}

// RunSpec defines the desired state of Run
type RunSpec struct {
	// +kubebuilder:validation:Enum={"apply","console","destroy","force-unlock","get","graph","init","import","output","plan","providers","providers lock","refresh","show","state list","state mv","state pull","state push","state replace-provider","state rm","state show","taint","untaint","validate","sh"}

	// The command to run on the pod
	Command string `json:"command"`

	// The arguments to be passed to the command
	Args []string `json:"args,omitempty"`

	// ConfigMap containing the tarball to extract on the pod
	ConfigMap string `json:"configMap"`

	// The config map key identifying the tarball to extract
	ConfigMapKey string `json:"configMapKey"`

	// The path within the archive to the root module
	ConfigMapPath string `json:"configMapPath"`

	// The workspace of the run.
	Workspace string `json:"workspace"`

	//+kubebuilder:validation:Minimum=0

	// Logging verbosity.
	Verbosity int `json:"verbosity,omitempty"`

	// AttachSpec defines behaviour for clients attaching to the pod's TTY
	AttachSpec `json:",inline"`
}

// AttachSpec defines behaviour for clients attaching to the pod's TTY
type AttachSpec struct {
	// Enable TTY on pod and await handshake string from client
	Handshake bool `json:"handshake,omitempty"`

	// +kubebuilder:default="10s"

	// How long to wait for handshake before timing out
	HandshakeTimeout string `json:"handshakeTimeout,omitempty"`
}

// ApprovedAnnotationKey is the key to be set on a workspace's annotations to
// indicate that this run is approved. Only necessary if the workspace has
// categorised the run's command as privileged.
func (r *Run) ApprovedAnnotationKey() string {
	return ApprovedAnnotationKey(r.Name)
}

func ApprovedAnnotationKey(runName string) string {
	return fmt.Sprintf("approvals.etok.dev/%s", runName)
}

// Run's pod shares its name
func (r *Run) PodName() string { return r.Name }

func (r *Run) LockFileConfigMapName() string {
	return RunLockFileConfigMapName(r.Name)
}

func RunLockFileConfigMapName(name string) string {
	return name + "-lockfile"
}

// RunStatus defines the observed state of Run
type RunStatus struct {
	// Current phase of the run's lifecycle.
	Phase RunPhase `json:"phase,omitempty"`

	// True if resource has been reconciled at least once.
	Reconciled bool `json:"reconciled,omitempty"`
}

func (r *Run) IsReconciled() bool {
	return r.RunStatus.Reconciled
}

type RunPhase string

const (
	RunPhaseUnknown      RunPhase = "unknown"
	RunPhasePending      RunPhase = "pending"
	RunPhaseQueued       RunPhase = "queued"
	RunPhaseProvisioning RunPhase = "provisioning"
	RunPhaseRunning      RunPhase = "running"
	RunPhaseCompleted    RunPhase = "completed"

	RunDefaultConfigMapKey = "config.tar.gz"
)
