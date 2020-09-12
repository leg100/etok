package v1alpha1

import (
	"github.com/operator-framework/operator-sdk/pkg/status"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func init() {
	SchemeBuilder.Register(&Run{}, &RunList{})
}

// Run is the Schema for the runs API
// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Command",type="string",JSONPath=".spec.Command",description="The run command"
// +kubebuilder:printcolumn:name="Workspace",type="string",JSONPath=".spec.workspace",description="The workspace of the run"
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase",description="The run phase"

type Run struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	RunSpec   `json:"spec,omitempty"`
	RunStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// RunList contains a list of Run
type RunList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Run `json:"items"`
}

// RunSpec defines the desired state of Run
type RunSpec struct {
	// +kubebuilder:validation:Enum={"apply","destroy","force-unlock","get","import","init","output","plan","refresh","sh","state list","state mv","state pull","state push","state rm","state show","taint","untaint","validate"}
	Command       string   `json:"command,omitempty"`
	Args          []string `json:"args,omitempty"`
	TimeoutClient string   `json:"timeoutClient"`
	TimeoutQueue  string   `json:"timeoutQueue"`
	Debug         bool     `json:"debug,omitempty"`
	ConfigMap     string   `json:"configMap"`
	ConfigMapKey  string   `json:"configMapKey"`
	Workspace     string   `json:"workspace"`
}

// Get/Set Args functions
func (c *RunSpec) GetArgs() []string     { return c.Args }
func (c *RunSpec) SetArgs(args []string) { c.Args = args }

// Get/Set TimeoutClient functions
func (c *RunSpec) GetTimeoutClient() string        { return c.TimeoutClient }
func (c *RunSpec) SetTimeoutClient(timeout string) { c.TimeoutClient = timeout }

// Get/Set TimeoutQueue functions
func (c *RunSpec) GetTimeoutQueue() string        { return c.TimeoutQueue }
func (c *RunSpec) SetTimeoutQueue(timeout string) { c.TimeoutQueue = timeout }

// Get/Set Debug functions
func (c *RunSpec) GetDebug() bool      { return c.Debug }
func (c *RunSpec) SetDebug(debug bool) { c.Debug = debug }

// Get/Set ConfigMap functions
func (c *RunSpec) GetConfigMap() string     { return c.ConfigMap }
func (c *RunSpec) SetConfigMap(name string) { c.ConfigMap = name }

// Get/Set ConfigMapKey functions
func (c *RunSpec) GetConfigMapKey() string    { return c.ConfigMapKey }
func (c *RunSpec) SetConfigMapKey(key string) { c.ConfigMapKey = key }

// Get/Set Workspace functions
func (c *RunSpec) GetWorkspace() string   { return c.Workspace }
func (c *RunSpec) SetWorkspace(ws string) { c.Workspace = ws }

// RunStatus defines the observed state of Run
type RunStatus struct {
	Conditions status.Conditions `json:"conditions,omitempty"`
	Phase      RunPhase          `json:"phase"`
}

type RunPhase string

func (c *RunStatus) GetConditions() *status.Conditions          { return &c.Conditions }
func (c *RunStatus) SetConditions(conditions status.Conditions) { c.Conditions = conditions }

// Get/Set Phase functions
func (c *RunStatus) GetPhase() RunPhase      { return c.Phase }
func (c *RunStatus) SetPhase(phase RunPhase) { c.Phase = phase }

const (
	RunPhasePending      RunPhase = "pending"
	RunPhaseQueued       RunPhase = "queued"
	RunPhaseProvisioning RunPhase = "provisioning"
	RunPhaseSync         RunPhase = "synchronising"
	RunPhaseRunning      RunPhase = "running"
	RunPhaseCompleted    RunPhase = "completed"

	ConditionCompleted status.ConditionType   = "Completed"
	ReasonPodCompleted status.ConditionReason = "PodCompleted"

	WaitAnnotationKey = "stok.goalspike.com/wait"

	RunDefaultConfigMapKey = "config.tar.gz"
)
