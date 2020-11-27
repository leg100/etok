package labels

import (
	"github.com/leg100/stok/version"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Label struct {
	Name, Value string
}

var (
	App                = Label{"app", "stok"}
	Version            = Label{"version", version.Version}
	OperatorComponent  = Component("operator")
	WorkspaceComponent = Component("workspace")
	RunComponent       = Component("run")
)

func Component(value string) Label {
	return Label{"component", value}
}

func Workspace(value string) Label {
	return Label{"workspace", value}
}

func Command(value string) Label {
	return Label{"command", value}
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
}
