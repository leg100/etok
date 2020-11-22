package v1alpha1

import (
	"fmt"
	"sort"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func init() {
	SchemeBuilder.Register(&Workspace{}, &WorkspaceList{})
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Workspace is the Schema for the workspaces API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=workspaces,scope=Namespaced
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
	// +kubebuilder:default=stok
	SecretName string `json:"secretName,omitempty"`
	// +kubebuilder:default=stok
	ServiceAccountName string `json:"serviceAccountName,omitempty"`

	Cache   WorkspaceCacheSpec `json:"cache,omitempty"`
	Backend BackendSpec        `json:"backend"`
	Debug   bool               `json:"debug,omitempty"`

	AttachSpec `json:",inline"`
}

// WorkspaceSpec defines the desired state of Workspace's cache storage
type WorkspaceCacheSpec struct {
	StorageClass string `json:"storageClass,omitempty"`
	Size         string `json:"size,omitempty" default:"1Gi"`
}

func (ws *Workspace) PodName() string {
	return WorkspacePodName(ws.GetName())
}

func (ws *Workspace) PVCName() string {
	return ws.GetName()
}

func (ws *Workspace) GetHandshake() bool          { return ws.Spec.AttachSpec.Handshake }
func (ws *Workspace) GetHandshakeTimeout() string { return ws.Spec.AttachSpec.HandshakeTimeout }

func (ws *Workspace) ContainerArgs() (args []string) {
	if ws.Spec.Debug {
		// Enable debug logging for the runner process
		args = append(args, "--debug")
	}

	// The runner process expects args to come after --
	args = append(args, "--")

	b := new(strings.Builder)
	b.WriteString("terraform init -backend-config=" + BackendConfigFilename)
	b.WriteString("; ")
	b.WriteString("terraform workspace select " + ws.GetNamespace() + "-" + ws.GetName())
	b.WriteString(" || ")
	b.WriteString("terraform workspace new " + ws.GetNamespace() + "-" + ws.GetName())

	args = append(args, []string{"sh", "-c", b.String()}...)

	return args
}

func (ws *Workspace) WorkingDir() string {
	return "/workspace"
}

func WorkspacePodName(name string) string {
	return "workspace-" + name
}

// Get/Set Debug functions
func (ws *Workspace) GetDebug() bool      { return ws.Spec.Debug }
func (ws *Workspace) SetDebug(debug bool) { ws.Spec.Debug = debug }

// WorkspaceStatus defines the observed state of Workspace
type WorkspaceStatus struct {
	Queue []string       `json:"queue"`
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

	WorkspaceDefaultSecretName         = "stok"
	WorkspaceDefaultServiceAccountName = "stok"

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
