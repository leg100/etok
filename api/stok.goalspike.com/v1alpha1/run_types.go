package v1alpha1

import (
	"fmt"
	"path/filepath"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	// +kubebuilder:validation:Enum={"apply","destroy","force-unlock","get","import","init","output","plan","refresh","sh","state list","state mv","state pull","state push","state rm","state show","taint","untaint","validate"}
	Command       string   `json:"command"`
	Args          []string `json:"args,omitempty"`
	Debug         bool     `json:"debug,omitempty"`
	ConfigMap     string   `json:"configMap"`
	ConfigMapKey  string   `json:"configMapKey"`
	ConfigMapPath string   `json:"configMapPath"`
	Workspace     string   `json:"workspace"`

	AttachSpec `json:",inline"`
}

// Get/Set Command functions
func (r *RunSpec) GetCommand() string    { return r.Command }
func (r *RunSpec) SetCommand(cmd string) { r.Command = cmd }

// Get/Set Args functions
func (r *RunSpec) GetArgs() []string     { return r.Args }
func (r *RunSpec) SetArgs(args []string) { r.Args = args }

// Get/Set Debug functions
func (r *RunSpec) GetDebug() bool      { return r.Debug }
func (r *RunSpec) SetDebug(debug bool) { r.Debug = debug }

// Get/Set ConfigMap functions
func (r *RunSpec) GetConfigMap() string     { return r.ConfigMap }
func (r *RunSpec) SetConfigMap(name string) { r.ConfigMap = name }

// Get/Set ConfigMapKey functions
func (r *RunSpec) GetConfigMapKey() string    { return r.ConfigMapKey }
func (r *RunSpec) SetConfigMapKey(key string) { r.ConfigMapKey = key }

// Get/Set ConfigMapPath functions
func (r *RunSpec) GetConfigMapPath() string     { return r.ConfigMapPath }
func (r *RunSpec) SetConfigMapPath(path string) { r.ConfigMapPath = path }

// Get/Set Workspace functions
func (r *RunSpec) GetWorkspace() string   { return r.Workspace }
func (r *RunSpec) SetWorkspace(ws string) { r.Workspace = ws }

func (r *Run) GetHandshake() bool          { return r.AttachSpec.Handshake }
func (r *Run) GetHandshakeTimeout() string { return r.AttachSpec.HandshakeTimeout }

func (r *Run) WorkingDir() string {
	return filepath.Join("/workspace", r.ConfigMapPath)
}

// ApprovedAnnotationKey is the key to be set on a workspace's annotations to
// indicate that this run is approved. Only necessary if the workspace has
// categorised the run's command as privileged.
func (r *Run) ApprovedAnnotationKey() string {
	return ApprovedAnnotationKey(r.Name)
}

func ApprovedAnnotationKey(runName string) string {
	return fmt.Sprintf("approvals.stok.goalspike.com/%s", runName)
}

// Run's pod shares its name
func (r *Run) PodName() string { return r.Name }

// ContainerArgs returns the args for a run's container
func (r *Run) ContainerArgs() (args []string) {
	if r.Debug {
		// Enable debug logging for the runner process
		args = append(args, "--debug")
	}

	// The runner process expects args to come after --
	args = append(args, "--")

	if r.Command != "sh" {
		// Any command other than sh is a terraform command
		args = append(args, "terraform")
	}

	args = append(args, strings.Split(r.Command, " ")...)
	args = append(args, r.Args...)

	return args
}

// RunStatus defines the observed state of Run
type RunStatus struct {
	Phase RunPhase `json:"phase"`
}

type RunPhase string

// Get/Set Phase functions
func (r *RunStatus) GetPhase() RunPhase      { return r.Phase }
func (r *RunStatus) SetPhase(phase RunPhase) { r.Phase = phase }

const (
	RunPhasePending      RunPhase = "pending"
	RunPhaseQueued       RunPhase = "queued"
	RunPhaseProvisioning RunPhase = "provisioning"
	RunPhaseRunning      RunPhase = "running"
	RunPhaseCompleted    RunPhase = "completed"

	RunDefaultConfigMapKey = "config.tar.gz"
)
