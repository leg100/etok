package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func init() {
	SchemeBuilder.Register(&CheckRun{}, &CheckList{})
}

// Check is the Schema for the checks API

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

type CheckRun struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CheckSpec   `json:"spec,omitempty"`
	Status CheckStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// CheckList contains a list of Check
type CheckList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CheckRun `json:"items"`
}

// CheckSpec defines the desired state of Check
type CheckSpec struct {
	CheckSuiteRef string `json:"checkSuite"`

	// The workspace of the check.
	Workspace string `json:"workspace"`
}

// CheckStatus defines the observed state of Check
type CheckStatus struct {
	// +kubebuilder:validation:Enum={"plan","apply"}
	Action string `json:"action"`

	// Can the check run be applied? Determines whether an 'apply' button is
	// shown.
	Appliable bool `json:"appliable"`

	Events []*CheckRunEvent `json:"events,omitempty"`

	// +kubebuilder:validation:Enum={"queued","in_progress","completed"}
	// +kubebuilder:default="queued"

	// The current status.
	Status string `json:"status,omitempty"`

	// +kubebuilder:validation:Enum={"success","failure","neutral","cancelled","timed_out","action_required"}

	// Optional. Required if you provide a status of "completed".
	Conclusion *string `json:"conclusion,omitempty"`
}

type CheckRunEvent struct {
	// Time event was received
	Received metav1.Time `json:"triggered"`

	Created         *CheckRunCreatedEvent         `json:"created,omitempty"`
	Rerequested     *CheckRunRerequestedEvent     `json:"rerequested,omitempty"`
	RequestedAction *CheckRunRequestedActionEvent `json:"requestedAction,omitempty"`
	Completed       *CheckRunCompletedEvent       `json:"completed,omitempty"`
}

// Github sends a created event after a github check run is created. It includes
// the ID of the created check run. The etok github app relies on this ID in
// order to work out which check run to update.
type CheckRunCreatedEvent struct {
	ID int64 `json:"id"`
}

// User re-requested that check run be re-run.
type CheckRunRerequestedEvent struct{}

// User requested that a specific action be carried out.
type CheckRunRequestedActionEvent struct {
	// +kubebuilder:validation:Enum={"plan","apply"}

	// The action that the user requested.
	Action string `json:"action"`
}

type CheckRunCompletedEvent struct{}
