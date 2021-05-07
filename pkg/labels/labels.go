package labels

import (
	"strings"

	"github.com/leg100/etok/pkg/version"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Label struct {
	Name, Value string
}

var (
	App                = Label{"app", "etok"}
	Version            = Label{"version", version.Version}
	Commit             = Label{"commit", version.Commit}
	OperatorComponent  = Component("operator")
	WorkspaceComponent = Component("workspace")
	RunComponent       = Component("run")
	WebhookComponent   = Component("webhook")
)

// A valid label must be an empty string or consist of alphanumeric characters ,
// '-', '_' or '.', and must start and end with an alphanumeric character (e.g.
// 'MyValue',  or 'my_value',  or '12345', regex used for validation is
// '(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?')
func NewLabel(name, value string) Label {
	return Label{
		Name:  strings.ReplaceAll(name, " ", "-"),
		Value: strings.ReplaceAll(value, " ", "-"),
	}
}

func Component(value string) Label {
	return NewLabel("component", value)
}

func Workspace(value string) Label {
	return NewLabel("workspace", value)
}

func Command(value string) Label {
	return NewLabel("command", value)
}

func SetLabel(obj metav1.Object, lbl Label) {
	labels := obj.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}
	labels[lbl.Name] = lbl.Value
	obj.SetLabels(labels)
}

func MakeLabels(lbl ...Label) map[string]string {
	labels := make(map[string]string)
	for _, l := range lbl {
		labels[l.Name] = l.Value
	}
	return labels
}

func SetCommonLabels(obj metav1.Object) {
	// Name of the application
	SetLabel(obj, App)
	// Current version of application
	SetLabel(obj, Version)
	// Current git commit of build
	SetLabel(obj, Commit)
}
