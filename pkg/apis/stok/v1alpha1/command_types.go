package v1alpha1

import (
	"strings"

	"github.com/iancoleman/strcase"
	"github.com/leg100/stok/pkg/apis/stok/v1alpha1/command"
	"github.com/operator-framework/operator-sdk/pkg/status"
	runtime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var CommandKinds = []string{
	"Apply",
	"Destroy",
	"ForceUnlock",
	"Get",
	"Import",
	"Init",
	"Output",
	"Plan",
	"Refresh",
	"Shell",
	"Show",
	"Taint",
	"Untaint",
	"Validate",
}

func NewCommandFromGVK(scheme *runtime.Scheme, gvk schema.GroupVersionKind) (command.Interface, error) {
	obj, err := scheme.New(gvk)
	if err != nil {
		return nil, err
	}
	return obj.(command.Interface), nil
}

// Convert k8s kind to stok CLI arg (e.g. Plan -> plan)
func CommandKindToCLI(kind string) string {
	return strcase.ToKebab(kind)
}

// Convert k8s kind to k8s API resource type (e.g. Plan -> plans)
func CommandKindToType(kind string) string {
	return strings.ToLower(kind) + "s"
}

// Convert stok CLI arg to k8s kind (e.g. plan -> Plan)
func CommandCLIToKind(cli string) string {
	return strcase.ToCamel(cli)
}

// Convert stok CLI arg to k8s resource type (e.g. plan -> plans)
func CommandCLIToType(cli string) string {
	return CommandKindToType(CommandCLIToKind(cli))
}

func CollectionKind(kind string) string {
	return kind + "List"
}

// For a given k8s kind and user supplied args, return the named program and args to be executed on
// the pod (e.g. plan -- -input=false -> terraform plan -input=false
func RunnerArgsForKind(kind string, args []string) []string {
	switch kind {
	case "Shell":
		// Wrap shell args into a single command string
		if len(args) > 0 {
			return []string{"/bin/sh", "-c", strings.Join(args, " ")}
		} else {
			return []string{"/bin/sh"}
		}
	default:
		// All other kinds are run as a terraform command, and the stok CLI name translates directly
		// to the terraform command name (e.g. stok plan -> terraform plan)
		return append([]string{"terraform", CommandKindToCLI(kind)}, args...)
	}
}

// CommandSpec defines the desired state of Command
type CommandSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html
	// +genclient
	Args          []string `json:"args,omitempty"`
	TimeoutClient string   `json:"timeoutclient"`
	TimeoutQueue  string   `json:"timeoutqueue"`
	Debug         bool     `json:"debug,omitempty"`
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

	CommandWaitAnnotationKey = "stok.goalspike.com/wait"
)
