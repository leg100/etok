package github

import (
	"context"
	"errors"
	"fmt"
	neturl "net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/leg100/etok/api/etok.dev/v1alpha1"
	"github.com/leg100/etok/pkg/client"
	"k8s.io/klog/v2"
)

// Clone git repo to dest, using token for auth, and checkout branch.
func clone(repo, dest, branch, token string) error {
	// Create dest dir and any ancestor dirs
	if err := os.MkdirAll(dest, 0700); err != nil {
		return fmt.Errorf("unable to make directory for repo: %s: %w", dest, err)
	}

	src, err := neturl.Parse(repo)
	if err != nil {
		return fmt.Errorf("unable to parse repo URL: %s: %w", repo, err)
	}
	src.User = neturl.UserPassword("x-access-token", token)

	args := []string{"clone", "--branch", branch, "--depth=1", "--single-branch", src.String(), dest}
	if _, err := runGitCmd(filepath.Dir(dest), args...); err != nil {
		// Redact token before propagating error
		return errors.New(strings.ReplaceAll(err.Error(), src.String(), src.Redacted()))
	}

	return nil
}

// reclone first removes a repo, if it exists, then re-creates it
func reclone(repo, dest, branch, token string) error {
	if err := os.RemoveAll(dest); err != nil {
		return fmt.Errorf("unable to remove git repo: %w", err)
	}
	return clone(repo, dest, branch, token)
}

// ensureCloned ensures that a repo is cloned to disk and that it is at the
// expected sha commit. If not it is re-cloned.
func ensureCloned(repo, dest, branch, sha, token string) error {
	_, err := os.Stat(dest)
	if os.IsNotExist(err) {
		return clone(repo, dest, branch, token)
	} else if err != nil {
		return fmt.Errorf("unable to check git repo exists: %w", err)
	}

	klog.V(1).Infof("clone directory %s already exists, checking if it's at the right commit", dest)

	output, err := runGitCmd(dest, "rev-parse", "HEAD")
	if err != nil {
		klog.Warningf("will re-clone repo, could not determine if was at correct commit: %s", err.Error())
		return reclone(repo, dest, branch, token)
	}
	currCommit := strings.Trim(output, "\n")

	if currCommit != sha {
		klog.V(1).Infof("repo was already cloned but is not at correct commit, wanted %s got %s", sha, currCommit)
		return reclone(repo, dest, branch, token)
	}

	klog.V(1).Infof("repo is at correct commit %s so will not re-clone", sha)
	return nil
}

func runGitCmd(path string, args ...string) (string, error) {
	cmd := exec.Command("git", args...) // nolint: gosec
	cmd.Dir = path

	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("unable to run git command (git %v): %w: %s", args, err, string(out))
	}
	return string(out), nil
}

// Path to cloned repo on disk
func clonePath(parent string, checkSuiteID int64) string {
	return filepath.Join(parent, strconv.FormatInt(checkSuiteID, 10))
}

// Path to workspace on disk
func workspacePath(parent string, checkSuiteID int64, workspaceWorkingDir string) string {
	return filepath.Join(clonePath(parent, checkSuiteID), workspaceWorkingDir)
}

// Get workspaces connected to the repo url
func getConnectedWorkspaces(client *client.Client, url string) (*v1alpha1.WorkspaceList, error) {
	connected := v1alpha1.WorkspaceList{}

	workspaces, err := client.WorkspacesClient("").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	for _, ws := range workspaces.Items {
		// Ignore workspaces connected to a different repo
		if ws.Spec.VCS.Repository != url {
			continue
		}
		connected.Items = append(connected.Items, ws)
	}

	return &connected, nil
}
