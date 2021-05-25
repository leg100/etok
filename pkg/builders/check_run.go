package builders

import (
	"fmt"
	"strconv"

	"github.com/leg100/etok/api/etok.dev/v1alpha1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type CheckRunBuilder struct {
	*v1alpha1.CheckRun

	workspace      string
	suite          int64
	suiteRerequest int
}

func CheckRun() *CheckRunBuilder {
	return &CheckRunBuilder{
		CheckRun: &v1alpha1.CheckRun{},
	}
}

func (b *CheckRunBuilder) Namespace(namespace string) *CheckRunBuilder {
	b.SetNamespace(namespace)
	return b
}

func (b *CheckRunBuilder) Suite(suite int64, rerequest int) *CheckRunBuilder {
	b.CheckRun.Spec.CheckSuiteRef = strconv.FormatInt(suite, 10)
	b.suite = suite
	b.suiteRerequest = rerequest
	return b
}

func (b *CheckRunBuilder) Workspace(workspace string) *CheckRunBuilder {
	b.CheckRun.Spec.Workspace = workspace
	b.workspace = workspace
	return b
}

func (b *CheckRunBuilder) Build() *v1alpha1.CheckRun {
	// CheckRun's name is composed of its CheckSuite, the CheckSuite ReRequest
	// number, and its Workspace, i.e.  "{suite}-{rerequest}-{workspace}"
	b.SetName(fmt.Sprintf("%d-%d-%s", b.suite, b.suiteRerequest, b.workspace))
	return b.CheckRun
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
