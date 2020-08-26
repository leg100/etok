// Code generated by go generate; DO NOT EDIT.
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Workspace",type="string",JSONPath=".spec.workspace",description="The workspace of the command"
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase",description="The command phase"

// Taint is the Schema for the taints API
type Taint struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	CommandSpec   `json:"spec,omitempty"`
	CommandStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// TaintList contains a list of Taint
type TaintList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Taint `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Taint{}, &TaintList{})
}
