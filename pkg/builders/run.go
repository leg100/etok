package builders

import (
	"time"

	"github.com/leg100/etok/api/etok.dev/v1alpha1"
	"github.com/leg100/etok/pkg/labels"
)

// Build a v1alpha1.Run resource using builder pattern
type RunBuilder struct {
	namespace, name string

	command string
	args    []string

	attach           bool
	handshakeTimeout time.Duration

	additionalLabels []labels.Label

	configMapPath string

	status    v1alpha1.RunStatus
	verbosity int
	workspace string
}

func Run(namespace, name, workspace, command string, args ...string) *RunBuilder {
	return &RunBuilder{
		name:             name,
		namespace:        namespace,
		workspace:        workspace,
		command:          command,
		args:             args,
		handshakeTimeout: v1alpha1.DefaultHandshakeTimeout,
	}
}

func (b *RunBuilder) Attach() *RunBuilder {
	b.attach = true
	return b
}

func (b *RunBuilder) SetLabel(name, value string) *RunBuilder {
	b.additionalLabels = append(b.additionalLabels, labels.Label{Name: name, Value: value})
	return b
}

// For testing purposes seed status
func (b *RunBuilder) SetStatus(status v1alpha1.RunStatus) *RunBuilder {
	b.status = status
	return b
}

func (b *RunBuilder) SetVerbosity(v int) *RunBuilder {
	b.verbosity = v
	return b
}

func (b *RunBuilder) Build() *v1alpha1.Run {
	run := v1alpha1.Run{}

	run.SetNamespace(b.namespace)
	run.SetName(b.name)

	// Set etok's common labels
	labels.SetCommonLabels(&run)

	// Permit filtering runs by command
	labels.SetLabel(&run, labels.Command(b.command))
	// Permit filtering runs by workspace
	labels.SetLabel(&run, labels.Workspace(b.workspace))
	// Permit filtering etok resources by component
	labels.SetLabel(&run, labels.RunComponent)

	for _, l := range b.additionalLabels {
		labels.SetLabel(&run, l)
	}

	run.Command = b.command
	run.Args = b.args

	run.Workspace = b.workspace

	// ConfigMap name is always set to same name as Run
	run.ConfigMap = b.name

	// And always set config map key to the default
	run.ConfigMapKey = v1alpha1.RunDefaultConfigMapKey

	run.Verbosity = b.verbosity

	if b.attach {
		run.AttachSpec.Handshake = true
		run.AttachSpec.HandshakeTimeout = b.handshakeTimeout.String()
	}

	run.RunStatus = b.status

	return &run
}
