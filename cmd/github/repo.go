package github

import (
	"errors"
	"fmt"
	neturl "net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/leg100/etok/api/etok.dev/v1alpha1"
	"k8s.io/klog/v2"
)

type tokenRefresher interface {
	refreshToken() (string, error)
}

type repo struct {
	url       string
	parentDir string
	branch    string
	sha       string
	owner     string
	name      string
}

func (r *repo) path() string {
	return filepath.Join(r.parentDir, r.sha)
}

func (r *repo) workspacePath(ws *v1alpha1.Workspace) string {
	return filepath.Join(r.path(), ws.Spec.VCS.WorkingDir)
}

func (r *repo) isCloned() (bool, error) {
	_, err := os.Stat(r.path())
	if os.IsNotExist(err) {
		return false, nil
	} else if err != nil {
		return false, fmt.Errorf("unable to check git repo exists: %w", err)
	}

	klog.V(1).Infof("clone directory %s already exists, checking if it's at the right commit", r.path())

	output, err := r.runGitCmd("rev-parse", "HEAD")
	if err != nil {
		klog.Warningf("will re-clone repo, could not determine if was at correct commit: %s", err.Error())
		return false, nil
	}
	currCommit := strings.Trim(output, "\n")

	if currCommit != r.sha {
		klog.V(1).Infof("repo was already cloned but is not at correct commit, wanted %s got %s", r.sha, currCommit)
		return false, nil
	}

	klog.V(1).Infof("repo is at correct commit %s so will not re-clone", r.sha)
	return true, nil
}

// Clone git repo to dest, using token for auth, and checkout branch.
func (r *repo) clone(refresher tokenRefresher) error {
	// Get access token for cloning repo
	token, err := refresher.refreshToken()
	if err != nil {
		return err
	}

	// Create repo dir and any ancestor dirs
	if err := os.MkdirAll(r.path(), 0700); err != nil {
		return fmt.Errorf("unable to make directory for repo: %s: %w", r.path(), err)
	}

	src, err := neturl.Parse(r.url)
	if err != nil {
		return fmt.Errorf("unable to parse repo URL: %w", err)
	}
	src.User = neturl.UserPassword("x-access-token", token)

	args := []string{"clone", "--branch", r.branch, "--depth=1", "--single-branch", src.String(), "."}
	if _, err := r.runGitCmd(args...); err != nil {
		// Redact token before propagating error
		return errors.New(strings.ReplaceAll(err.Error(), src.String(), src.Redacted()))
	}

	return nil
}

// reclone first removes a repo, if it exists, then re-creates it
func (r *repo) reclone(refresher tokenRefresher) error {
	if err := os.RemoveAll(r.path()); err != nil {
		return fmt.Errorf("unable to remove git repo: %w", err)
	}
	return r.clone(refresher)
}

// ensureCloned ensures that a repo is cloned to disk and that it is at the
// expected sha commit. If not it is re-cloned.
func (r *repo) ensureCloned(refresher tokenRefresher) error {
	cloned, err := r.isCloned()
	if err != nil {
		return err
	}
	if cloned {
		return nil
	}
	return r.reclone(refresher)
}

func (r *repo) runGitCmd(args ...string) (string, error) {
	cmd := exec.Command("git", args...) // nolint: gosec
	cmd.Dir = r.path()

	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("unable to run git command (git %v): %w: %s", args, err, string(out))
	}
	return string(out), nil
}
