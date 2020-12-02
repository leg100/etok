package v1alpha1

import (
	"fmt"
	"sort"
	"strings"

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
// +kubebuilder:printcolumn:name="Queue",type="string",JSONPath=".status.queue"
// +kubebuilder:printcolumn:name="Backend",type="string",JSONPath=".spec.backend.type"
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
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
	SecretName string `json:"secretName,omitempty"`
	// +kubebuilder:default=etok
	ServiceAccountName string `json:"serviceAccountName,omitempty"`

	Cache   WorkspaceCacheSpec `json:"cache,omitempty"`
	Backend BackendSpec        `json:"backend"`

	Verbosity int `json:"verbosity,omitempty"`

	PrivilegedCommands []string `json:"privilegedCommands"`

	// TODO: kubebuilder markers
	TerraformVersion string `json:"terraformVersion"`

	AttachSpec `json:",inline"`
}

// WorkspaceSpec defines the desired state of Workspace's cache storage
type WorkspaceCacheSpec struct {
	StorageClass string `json:"storageClass,omitempty"`
	Size         string `json:"size,omitempty" default:"1Gi"`
}

func (ws *Workspace) PodName() string {
	return WorkspacePodName(ws.Name)
}

func (ws *Workspace) TerraformName() string {
	return fmt.Sprintf("%s-%s", ws.Namespace, ws.Name)
}

func (ws *Workspace) PVCName() string {
	return ws.Name
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
	Queue []string       `json:"queue,omitempty"`
	Phase WorkspacePhase `json:"phase"`
}

type WorkspacePhase string

const (
	WorkspacePhaseInitializing WorkspacePhase = "initializing"
	WorkspacePhaseReady        WorkspacePhase = "ready"
	WorkspacePhaseError        WorkspacePhase = "error"
	WorkspacePhaseUnknown      WorkspacePhase = "unknown"
	WorkspacePhaseDeleting     WorkspacePhase = "deleting"

	WorkspaceDefaultCacheSize = "1Gi"

	WorkspaceDefaultSecretName         = "etok"
	WorkspaceDefaultServiceAccountName = "etok"

	BackendTypeFilename   = "backend.tf"
	BackendConfigFilename = "backend.ini"
)

type BackendSpec struct {
	// +kubebuilder:validation:Enum=local;remote;artifactory;azurerm;consul;cos;etcd;etcdv3;gcs;http;manta;oss;pg;s3;swift
	Type   string            `json:"type,omitempty"`
	Config map[string]string `json:"config,omitempty"`
}

func BackendEmptyConfig(backendType string) string {
	return fmt.Sprintf(`terraform {
  backend "%s" {}
}
`, backendType)
}

// Return a terraform backend configuration file (similar to an INI file)
func BackendConfig(cfg map[string]string) string {
	// Sort keys into a slice, otherwise tests occasionally fail as a result of go's random iteration
	// of maps
	keys := make([]string, 0, len(cfg))
	for k := range cfg {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	for _, k := range keys {
		fmt.Fprintf(&b, "%s\t= \"%s\"\n", k, cfg[k])
	}
	return b.String()
}

func BackendConfigMapName(workspace string) string {
	return "workspace-" + workspace
}
