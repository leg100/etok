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

func CheckSuite(id int64) *checkSuiteBuilder {
	suite := &v1alpha1.CheckSuite{}
	suite.SetName(strconv.FormatInt(id, 10))

	return &checkSuiteBuilder{suite}
}

func CheckSuiteFromEvent(ev *github.CheckSuiteEvent) *checkSuiteBuilder {
	suite := &v1alpha1.CheckSuite{
		ObjectMeta: metav1.ObjectMeta{
			Name: strconv.FormatInt(ev.CheckSuite.GetID(), 10),
		},
		Spec: v1alpha1.CheckSuiteSpec{
			ID:        ev.GetCheckSuite().GetID(),
			CloneURL:  ev.GetRepo().GetCloneURL(),
			InstallID: ev.GetInstallation().GetID(),
			SHA:       ev.GetCheckSuite().GetHeadSHA(),
			Owner:     ev.GetRepo().GetOwner().GetLogin(),
			Repo:      ev.GetRepo().GetName(),
			Branch:    ev.GetCheckSuite().GetHeadBranch(),
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
			ID:       obj.GetID(),
			CloneURL: obj.GetRepository().GetCloneURL(),
			SHA:      obj.GetHeadSHA(),
			Owner:    obj.GetRepository().GetOwner().GetLogin(),
			Repo:     obj.GetRepository().GetName(),
			Branch:   obj.GetHeadBranch(),
		},
	}
	return &checkSuiteBuilder{suite}
}

func (b *checkSuiteBuilder) CloneURL(url string) *checkSuiteBuilder {
	b.Spec.CloneURL = url
	return b
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
