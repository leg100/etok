package github

type event interface {
	GetHeadSHA() string
	GetHeadBranch() string
	GetID() int64
}
