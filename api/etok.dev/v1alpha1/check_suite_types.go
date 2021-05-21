package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func init() {
	SchemeBuilder.Register(&CheckSuite{}, &CheckSuiteList{})
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=checksuites,scope=Cluster,shortName={suite}

// CheckSuite is the Schema for the checksuite API
type CheckSuite struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CheckSuiteSpec   `json:"spec,omitempty"`
	Status CheckSuiteStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// CheckSuiteList contains a list of CheckSuite
type CheckSuiteList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CheckSuite `json:"items"`
}

// CheckSuiteSpec defines the desired state of CheckSuite
type CheckSuiteSpec struct {
	Branch string `json:"branch"`

	SHA string `json:"sha"`

	Owner string `json:"owner"`

	Repo string `json:"repo"`

	CloneURL string `json:"cloneURL"`

	InstallID int64 `json:"installID"`
}

// CheckSuiteStatus defines the observed state of CheckSuite
type CheckSuiteStatus struct {
	RepoPath string `json:"repoPath"`
}
