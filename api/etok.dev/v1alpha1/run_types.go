package v1alpha1

import (
	"fmt"

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
	// +kubebuilder:validation:Enum={"apply","destroy","plan","sh"}
	Command       string   `json:"command"`
	Args          []string `json:"args,omitempty"`
	ConfigMap     string   `json:"configMap"`
	ConfigMapKey  string   `json:"configMapKey"`
	ConfigMapPath string   `json:"configMapPath"`
	Workspace     string   `json:"workspace"`

	Verbosity int `json:"verbosity,omitempty"`

	AttachSpec `json:",inline"`
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

// RunStatus defines the observed state of Run
type RunStatus struct {
	Phase RunPhase `json:"phase"`
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
