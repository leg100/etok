package v1alpha1

import (
	"github.com/operator-framework/operator-sdk/pkg/status"
)

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// CommandSpec defines the desired state of Command
type CommandSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html
	// +genclient
	Args []string `json:"args,omitempty"`
}

func (c *CommandSpec) GetArgs() []string {
	return c.Args
}

func (c *CommandSpec) SetArgs(args []string) {
	c.Args = args
}

// CommandStatus defines the observed state of Command
type CommandStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html
	// +genclient
	Conditions status.Conditions `json:"conditions"`
	Phase      string            `json:"phase"`
}

func (c *CommandStatus) GetConditions() *status.Conditions {
	return &c.Conditions
}

func (c *CommandStatus) SetConditions(conditions status.Conditions) {
	c.Conditions = conditions
}
