package v1alpha1

import (
	"github.com/operator-framework/operator-sdk/pkg/status"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// CommandSpec defines the desired state of Command
type CommandSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html
	Command []string `json:"command,omitempty"`

	Args         []string `json:"args,omitempty"`
	ConfigMap    string   `json:"configmap"`
	ConfigMapKey string   `json:"configmapkey"`
}

// CommandStatus defines the observed state of Command
type CommandStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html
	Conditions status.Conditions `json:"conditions"`
	Phase      string            `json:"phase"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Command is the Schema for the commands API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=commands,scope=Namespaced
// +genclient
type Command struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CommandSpec   `json:"spec,omitempty"`
	Status CommandStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CommandList contains a list of Command
type CommandList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Command `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Command{}, &CommandList{})
}
