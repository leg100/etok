package builders

import (
	"strings"

	"github.com/leg100/etok/api/etok.dev/v1alpha1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type CheckRunBuilder struct {
	*v1alpha1.CheckRun
}

// key follows the format {namespace}/{name}
func CheckRun(key string) *CheckRunBuilder {
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

	return &CheckRunBuilder{cr}
}

// For testing purposes
func (b *CheckRunBuilder) CreateRequested() *CheckRunBuilder {
	meta.SetStatusCondition(&b.Status.Conditions, metav1.Condition{
		Type:    "CreateRequested",
		Status:  metav1.ConditionTrue,
		Reason:  "CreateRequestSent",
		Message: "Create request has been sent to the Github API",
	})
	return b
}

// For testing purposes
func (b *CheckRunBuilder) ID(id int64) *CheckRunBuilder {
	b.Status.Events = append(b.Status.Events, &v1alpha1.CheckRunEvent{
		Created: &v1alpha1.CheckRunCreatedEvent{
			ID: id,
		},
	})
	return b
}

// CheckRun's name is composed of its CheckSuite and its Workspace, i.e.
// "{suite}-{workspace}"
func parseCheckRunName(name string) (suite string, ws string) {
	parts := strings.Split(name, "-")
	return parts[0], parts[1]
}

func (b *CheckRunBuilder) Build() *v1alpha1.CheckRun {
	return b.CheckRun
}
