package v1alpha1

import (
	"github.com/leg100/etok/pkg/util/slice"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func init() {
	SchemeBuilder.Register(&Workspace{}, &WorkspaceList{})
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Workspace is the Schema for the workspaces API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=workspaces,scope=Namespaced,shortName={ws}
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Version",type="string",JSONPath=".spec.terraformVersion"
// +kubebuilder:printcolumn:name="Queue",type="string",JSONPath=".status.queue"
// +genclient
type Workspace struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WorkspaceSpec   `json:"spec,omitempty"`
	Status WorkspaceStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// WorkspaceList contains a list of Workspace
type WorkspaceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Workspace `json:"items"`
}

// WorkspaceSpec defines the desired state of Workspace
type WorkspaceSpec struct {
	// +kubebuilder:default=etok

	// Name of the secret resource. Its keys and values will be made available
	// as environment variables on the workspace pod.
	SecretName string `json:"secretName,omitempty"`

	// +kubebuilder:default=etok

	// Name of the service account configured for the workspace pod.
	ServiceAccountName string `json:"serviceAccountName,omitempty"`

	// Persistent Volume Claim specification for workspace's cache.
	Cache WorkspaceCacheSpec `json:"cache,omitempty"`

	//+kubebuilder:validation:Minimum=0

	// Logging verbosity.
	Verbosity int `json:"verbosity,omitempty"`

	// List of commands that are deemed privileged. The client must set a
	// specific annotation on the workspace to approve a run with a privileged
	// command.
	PrivilegedCommands []string `json:"privilegedCommands,omitempty"`

	// Any change to the default marker for the terraform version below must
	// also be made to the dockerfile for the container image
	// (/build/Dockerfile)

	// +kubebuilder:default="0.14.3"
	// +kubebuilder:validation:Pattern=`[0-9]+\.[0-9]+\.[0-9]+`

	// Required version of Terraform on workspace pod
	TerraformVersion string `json:"terraformVersion,omitempty"`
}

// WorkspaceSpec defines the desired state of Workspace's cache storage
type WorkspaceCacheSpec struct {
	// Storage class for the cache's persistent volume claim. This is a pointer
	// to distinguish between explicit empty string and nil (which triggers
	// different behaviour for dynamic provisioning of persistent volumes).
	StorageClass *string `json:"storageClass,omitempty"`

	// +kubebuilder:default="1Gi"

	// Size of cache's persistent volume claim.
	Size string `json:"size,omitempty"`
}

func (ws *Workspace) PodName() string {
	return WorkspacePodName(ws.Name)
}

func (ws *Workspace) PVCName() string {
	return ws.Name
}

func (ws *Workspace) VariablesConfigMapName() string {
	return ws.Name + "-variables"
}

func (ws *Workspace) IsPrivilegedCommand(cmd string) bool {
	return slice.ContainsString(ws.Spec.PrivilegedCommands, cmd)
}

func (ws *Workspace) IsRunApproved(run *Run) bool {
	if annotations := ws.Annotations; annotations != nil {
		status, exists := annotations[run.ApprovedAnnotationKey()]
		if exists && status == "approved" {
			return true
		}
	}
	return false
}

func WorkspacePodName(name string) string {
	return "workspace-" + name
}

// WorkspaceStatus defines the observed state of Workspace
type WorkspaceStatus struct {
	// Queue of runs. Parse bypass the queue.
	Queue []string `json:"queue,omitempty"`

	// Lifecycle phase of workspace.
	Phase WorkspacePhase `json:"phase,omitempty"`

	// True if resource has been reconciled at least once.
	Reconciled bool `json:"reconciled,omitempty"`
}

func (ws *Workspace) IsReconciled() bool {
	return ws.Status.Reconciled
}

type WorkspacePhase string

const (
	WorkspacePhaseInitializing WorkspacePhase = "initializing"
	WorkspacePhaseReady        WorkspacePhase = "ready"
	WorkspacePhaseError        WorkspacePhase = "error"
	WorkspacePhaseUnknown      WorkspacePhase = "unknown"
	WorkspacePhaseDeleting     WorkspacePhase = "deleting"
)
