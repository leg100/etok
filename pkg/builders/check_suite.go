package builders

import (
	"github.com/leg100/etok/api/etok.dev/v1alpha1"
)

type checkSuiteBuilder struct {
	*v1alpha1.CheckSuite
}

// key follows the format {namespace}/{name}
func CheckSuite(name string) *checkSuiteBuilder {
	suite := &v1alpha1.CheckSuite{}
	suite.SetName(name)

	return &checkSuiteBuilder{suite}
}

func (b *checkSuiteBuilder) RepoPath(path string) *checkSuiteBuilder {
	b.Status.RepoPath = path
	return b
}

func (b *checkSuiteBuilder) Build() *v1alpha1.CheckSuite {
	return b.CheckSuite
}
