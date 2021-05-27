package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func init() {
	SchemeBuilder.Register(&CheckRun{}, &CheckRunList{})
}

// Check is the Schema for the checks API

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=checkruns,scope=Namespaced,shortName={check}

type CheckRun struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CheckRunSpec   `json:"spec,omitempty"`
	Status CheckRunStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// CheckRunList contains a list of Check
type CheckRunList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CheckRun `json:"items"`
}

// CheckRunSpec defines the desired state of Check
type CheckRunSpec struct {
	CheckSuiteRef CheckSuiteRef `json:"checkSuiteRef"`

	// The workspace of the check.
	Workspace string `json:"workspace"`
}

// CheckSuiteRef defines a CheckRun's reference to a CheckSuite
type CheckSuiteRef struct {
	// Name of the CheckSuite resource
	Name string `json:"name"`

	// The rerequest number that spawned the referencing CheckRun
	RerequestNumber int `json:"rerequestNumber,omitEmpty"`
}

// CheckRunStatus defines the observed state of Check
type CheckRunStatus struct {
	Events []*CheckRunEvent `json:"events,omitempty"`

	Iterations []*CheckRunIteration `json:"iterations,omitempty"`

	// +kubebuilder:validation:Enum={"queued","in_progress","completed"}
	// +kubebuilder:default="queued"

	// The current status.
	Status string `json:"status,omitempty"`

	// +kubebuilder:validation:Enum={"success","failure","neutral","cancelled","timed_out","action_required"}

	// Optional. Required if you provide a status of "completed".
	Conclusion *string `json:"conclusion,omitempty"`

	Conditions []metav1.Condition `json:"conditions,omitempty"`
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

type CheckRunIteration struct {
	// Etok run triggered in this iteration
	Run string `json:"runName"`

	// Whether this iteration has completed
	Completed bool `json:"completed,omitempty"`
}
