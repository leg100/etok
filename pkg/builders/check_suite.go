package builders

import (
	"strconv"

	"github.com/google/go-github/v31/github"
	"github.com/leg100/etok/api/etok.dev/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type checkSuiteBuilder struct {
	*v1alpha1.CheckSuite
}

func CheckSuite(name string) *checkSuiteBuilder {
	suite := &v1alpha1.CheckSuite{}
	suite.SetName(name)

	return &checkSuiteBuilder{suite}
}

func CheckSuiteFromEvent(ev *github.CheckSuiteEvent) *checkSuiteBuilder {
	suite := &v1alpha1.CheckSuite{
		ObjectMeta: metav1.ObjectMeta{
			Name: strconv.FormatInt(ev.CheckSuite.GetID(), 10),
		},
		Spec: v1alpha1.CheckSuiteSpec{
			CloneURL:  ev.Repo.GetCloneURL(),
			InstallID: ev.GetInstallation().GetID(),
			SHA:       ev.CheckSuite.GetHeadSHA(),
			Owner:     ev.Repo.Owner.GetLogin(),
			Repo:      ev.Repo.GetName(),
			Branch:    ev.CheckSuite.GetHeadBranch(),
		},
	}
	return &checkSuiteBuilder{suite}
}

func CheckSuiteFromObj(obj *github.CheckSuite) *checkSuiteBuilder {
	suite := &v1alpha1.CheckSuite{
		ObjectMeta: metav1.ObjectMeta{
			Name: strconv.FormatInt(obj.GetID(), 10),
		},
		Spec: v1alpha1.CheckSuiteSpec{
			CloneURL: obj.GetRepository().GetCloneURL(),
			SHA:      obj.GetHeadSHA(),
			Owner:    obj.App.Owner.GetLogin(),
			Repo:     obj.GetRepository().GetName(),
			Branch:   obj.GetHeadBranch(),
		},
	}
	return &checkSuiteBuilder{suite}
}

func (b *checkSuiteBuilder) InstallID(id int64) *checkSuiteBuilder {
	b.Spec.InstallID = id
	return b
}

func (b *checkSuiteBuilder) RepoPath(path string) *checkSuiteBuilder {
	b.Status.RepoPath = path
	return b
}

func (b *checkSuiteBuilder) Build() *v1alpha1.CheckSuite {
	return b.CheckSuite
}
