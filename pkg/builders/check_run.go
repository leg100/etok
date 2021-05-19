package builders

import (
	"strings"

	"github.com/leg100/etok/api/etok.dev/v1alpha1"
)

type checkRunBuilder struct {
	*v1alpha1.CheckRun
}

// key follows the format {namespace}/{name}
func CheckRun(key string) *checkRunBuilder {
	var namespace, name string
	cr := &v1alpha1.CheckRun{}

	parts := strings.Split(key, "/")
	if len(parts) > 1 {
		namespace = parts[0]
		name = parts[1]
	} else {
		name = parts[0]
	}
	cr.SetNamespace(namespace)
	cr.SetName(name)

	cr.Spec.CheckSuiteRef, cr.Spec.Workspace = parseCheckRunName(name)

	return &checkRunBuilder{cr}
}

// CheckRun's name is composed of its CheckSuite and its Workspace, i.e.
// "{suite}-{workspace}"
func parseCheckRunName(name string) (suite string, ws string) {
	parts := strings.Split(name, "-")
	return parts[0], parts[1]
}

func (b *checkRunBuilder) Build() *v1alpha1.CheckRun {
	return b.CheckRun
}
