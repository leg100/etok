package v1alpha1

import (
	"github.com/operator-framework/operator-sdk/pkg/status"
)

// CommandSpec defines the desired state of Command
type CommandSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html
	// +genclient
	Args          []string `json:"args,omitempty"`
	TimeoutClient string   `json:"timeoutClient"`
	TimeoutQueue  string   `json:"timeoutQueue"`
	Debug         bool     `json:"debug,omitempty"`
	ConfigMap     string   `json:"configMap"`
	ConfigMapKey  string   `json:"configMapKey"`
	Workspace     string   `json:"workspace"`
}

// Get/Set Args functions
func (c *CommandSpec) GetArgs() []string     { return c.Args }
func (c *CommandSpec) SetArgs(args []string) { c.Args = args }

// Get/Set TimeoutClient functions
func (c *CommandSpec) GetTimeoutClient() string        { return c.TimeoutClient }
func (c *CommandSpec) SetTimeoutClient(timeout string) { c.TimeoutClient = timeout }

// Get/Set TimeoutQueue functions
func (c *CommandSpec) GetTimeoutQueue() string        { return c.TimeoutQueue }
func (c *CommandSpec) SetTimeoutQueue(timeout string) { c.TimeoutQueue = timeout }

// Get/Set Debug functions
func (c *CommandSpec) GetDebug() bool      { return c.Debug }
func (c *CommandSpec) SetDebug(debug bool) { c.Debug = debug }

// Get/Set ConfigMap functions
func (c *CommandSpec) GetConfigMap() string     { return c.ConfigMap }
func (c *CommandSpec) SetConfigMap(name string) { c.ConfigMap = name }

// Get/Set ConfigMapKey functions
func (c *CommandSpec) GetConfigMapKey() string    { return c.ConfigMapKey }
func (c *CommandSpec) SetConfigMapKey(key string) { c.ConfigMapKey = key }

// Get/Set Workspace functions
func (c *CommandSpec) GetWorkspace() string   { return c.Workspace }
func (c *CommandSpec) SetWorkspace(ws string) { c.Workspace = ws }

// CommandStatus defines the observed state of Command
type CommandStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html
	// +genclient
	Conditions status.Conditions `json:"conditions"`
}

func (c *CommandStatus) GetConditions() *status.Conditions          { return &c.Conditions }
func (c *CommandStatus) SetConditions(conditions status.Conditions) { c.Conditions = conditions }

const (
	ConditionCompleted   status.ConditionType = "Completed"
	ConditionClientReady status.ConditionType = "ClientReady"
	ConditionAttachable  status.ConditionType = "PodAttachable"

	ReasonWorkspaceUnspecified status.ConditionReason = "WorkspaceUnspecified"
	ReasonWorkspaceNotFound    status.ConditionReason = "WorkspaceNotFound"
	ReasonUnscheduled          status.ConditionReason = "Unscheduled"
	ReasonQueued               status.ConditionReason = "InWorkspaceQueue"
	ReasonPodRunningAndReady   status.ConditionReason = "PodRunningAndReady"
	ReasonClientAttached       status.ConditionReason = "ClientAttached"
	ReasonPodCompleted         status.ConditionReason = "PodCompleted"

	WaitAnnotationKey = "stok.goalspike.com/wait"

	CommandDefaultConfigMapKey = "config.tar.gz"

	// ConfigMap/etcd only supports data payload of up to 1MB, which limits the size of
	// tf config that can be uploaded (after compression).
	// thttps://github.com/kubernetes/kubernetes/issues/19781
	MaxConfigSize = 1024 * 1024
)
