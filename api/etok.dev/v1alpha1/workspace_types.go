package v1alpha1

import (
	"fmt"

	"github.com/leg100/etok/pkg/util/slice"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
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
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="Active",type="string",JSONPath=".status.active"
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
	// +kubebuilder:validation:Pattern=`^[0-9]+\.[0-9]+\.[0-9]+$`

	// Required version of Terraform on workspace pod
	TerraformVersion string `json:"terraformVersion,omitempty"`

	// Variables as inputs to module
	Variables []*Variable `json:"variables,omitempty"`

	// +kubebuilder:validation:Pattern=`^[0-9a-z][0-9a-z\-_]{0,61}[0-9a-z]$`

	// GCS bucket to which to backup state file
	BackupBucket string `json:"backupBucket,omitempty"`
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

// WorkspaceStatus defines the observed state of Workspace
type WorkspaceStatus struct {
	// Queue of runs. Only runs with queueable commands (sh, apply, etc) are
	// queued.
	Queue []string `json:"queue,omitempty"`

	Active string `json:"active,omitempty"`

	// Lifecycle phase of workspace.
	Phase WorkspacePhase `json:"phase,omitempty"`

	// Outputs from state file
	Outputs []*Output `json:"outputs,omitempty"`

	// Serial number of state file. Nil means there is no state file.
	Serial *int `json:"serial,omitempty"`

	// Serial number of the last successfully backed up state file. Nil means it
	// has not been backed up.
	BackupSerial *int `json:"backupSerial,omitempty"`

	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// Variable denotes an input to the module
type Variable struct {
	// Variable name
	Key string `json:"key"`
	// Variable value
	Value string `json:"value"`
	// Source for the variable's value. Cannot be used if value is not empty.
	ValueFrom *corev1.EnvVarSource `json:"valueFrom,omitempty"`
	// EnvironmentVariable denotes if this variable should be created as
	// environment variable
	EnvironmentVariable bool `json:"environmentVariable,omitempty"`
}

// Output outputs the values of Terraform output
type Output struct {
	// Attribute name in module
	Key string `json:"key"`
	// Value
	Value string `json:"value"`
}

// IsReconciled indicates whether resource has reconciled. It does this by
// checking that a ready condition has been set, regardless of whether it is
// true or false.
func (ws *Workspace) IsReconciled() bool {
	ready := meta.FindStatusCondition(ws.Status.Conditions, WorkspaceReadyCondition)

	if ready != nil {
		return true
	}
	return false
}

func (ws *Workspace) PodName() string {
	return WorkspacePodName(ws.Name)
}

func (ws *Workspace) PVCName() string {
	return ws.Name
}

// StateSecretName retrieves the name of the secret containing the terraform
// state for this workspace.
func (ws *Workspace) StateSecretName() string {
	return fmt.Sprintf("tfstate-default-%s", ws.Name)
}

// BackupObjectName returns the object name to be used for the backup of the
// workspace's state file.
func (ws *Workspace) BackupObjectName() string {
	return fmt.Sprintf("%s/%s.yaml", ws.Namespace, ws.Name)
}

func (ws *Workspace) VariablesConfigMapName() string {
	return WorkspaceVariablesConfigMapName(ws.Name)
}

func WorkspaceVariablesConfigMapName(name string) string {
	return name + "-variables"
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

type WorkspacePhase string

const (
	WorkspacePhaseInitializing WorkspacePhase = "initializing"
	WorkspacePhaseReady        WorkspacePhase = "ready"
	WorkspacePhaseError        WorkspacePhase = "error"
	WorkspacePhaseUnknown      WorkspacePhase = "unknown"
	WorkspacePhaseDeleting     WorkspacePhase = "deleting"
)
