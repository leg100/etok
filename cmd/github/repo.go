package github

import (
	"path/filepath"
	"time"

	"github.com/leg100/etok/api/etok.dev/v1alpha1"
)

// Repo represents a local clone of repo
type repo struct {
	url        string
	branch     string
	sha        string
	owner      string
	name       string
	path       string
	lastCloned time.Time
}

func (r *repo) workspacePath(ws *v1alpha1.Workspace) string {
	return filepath.Join(r.path, ws.Spec.VCS.WorkingDir)
}
