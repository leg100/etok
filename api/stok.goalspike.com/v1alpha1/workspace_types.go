package v1alpha1

import (
	"fmt"
	"sort"
	"strings"

	"github.com/creasty/defaults"
	"github.com/operator-framework/operator-sdk/pkg/status"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// WorkspaceSpec defines the desired state of Workspace's cache storage
type WorkspaceCacheSpec struct {
	StorageClass string `json:"storageClass,omitempty"`
	Size         string `json:"size,omitempty" default:"1Gi"`
}

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

func (ws *Workspace) PodName() string {
	return WorkspacePodName(ws.GetName())
}

func WorkspacePodName(name string) string {
	return "workspace-" + name
}

// WorkspaceSpec defines the desired state of Workspace
type WorkspaceSpec struct {
	SecretName         string `json:"secretName,omitempty"`
	ServiceAccountName string `json:"serviceAccountName,omitempty"`

	Cache         WorkspaceCacheSpec `json:"cache,omitempty"`
	Backend       BackendSpec        `json:"backend"`
	TimeoutClient string             `json:"timeoutClient"`
	Debug         bool               `json:"debug,omitempty"`

	AttachSpec `json:"inline"`
}

func (w *WorkspaceSpec) SetDefaults() {
	if defaults.CanUpdate(w.SecretName) {
		w.SecretName = "stok"
	}
	if defaults.CanUpdate(w.ServiceAccountName) {
		w.ServiceAccountName = "stok"
	}
}

// Get/Set TimeoutQueue functions
func (ws *Workspace) GetTimeoutClient() string        { return ws.Spec.TimeoutClient }
func (ws *Workspace) SetTimeoutClient(timeout string) { ws.Spec.TimeoutClient = timeout }

// Get/Set Debug functions
func (ws *Workspace) GetDebug() bool      { return ws.Spec.Debug }
func (ws *Workspace) SetDebug(debug bool) { ws.Spec.Debug = debug }

// WorkspaceStatus defines the observed state of Workspace
type WorkspaceStatus struct {
	Queue      []string          `json:"queue"`
	Conditions status.Conditions `json:"conditions,omitempty"`
}

const (
	ConditionHealthy status.ConditionType = "Healthy"

	ReasonAllResourcesFound status.ConditionReason = "AllResourcesFound"
	ReasonMissingResource   status.ConditionReason = "MissingResource"

	WorkspaceDefaultCacheSize = "1Gi"

	WorkspaceDefaultSecretName         = "stok"
	WorkspaceDefaultServiceAccountName = "stok"

	BackendTypeFilename   = "backend.tf"
	BackendConfigFilename = "backend.ini"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Workspace is the Schema for the workspaces API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=workspaces,scope=Namespaced
// +kubebuilder:printcolumn:name="Queue",type="string",JSONPath=".status.queue"
// +kubebuilder:printcolumn:name="Backend",type="string",JSONPath=".spec.backend.type"
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

func init() {
	SchemeBuilder.Register(&Workspace{}, &WorkspaceList{})
}
