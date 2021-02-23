package github

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/google/go-github/v31/github"
	"github.com/leg100/etok/api/etok.dev/v1alpha1"
	"github.com/leg100/etok/pkg/client"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

type webhookCheckSuite struct {
	*client.Client
	ghClient   *GithubClient
	checkSuite *github.CheckSuiteEvent
	cloneDir   string
}

func newWebhookCheckSuite(c *client.Client, ghClient *GithubClient, cloneDir string, ev *github.CheckSuiteEvent) *webhookCheckSuite {
	return &webhookCheckSuite{
		checkSuite: ev,
		cloneDir:   cloneDir,
		ghClient:   ghClient,
		Client:     c,
	}
}

func (o *webhookCheckSuite) run() error {
	// Get fresh access token for cloning repo
	token, err := o.ghClient.refreshToken()
	if err != nil {
		return err
	}

	path := filepath.Join(o.cloneDir, strconv.FormatInt(*o.checkSuite.CheckSuite.ID, 10))

	if err := o.cloneRepo(token, path); err != nil {
		return err
	}

	// Enumerate workspaces and retrieve 'connected' workspaces.
	workspaces, err := o.WorkspacesClient("").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		panic(err.Error())
	}

	workspaces = filterWorkspaces(workspaces, *o.checkSuite.Repo.CloneURL, path)

	// We now know how many check runs to create

	for _, ws := range workspaces.Items {
		checkRun, _, err := o.ghClient.Checks.CreateCheckRun(context.Background(), *o.checkSuite.Repo.Owner.Login, *o.checkSuite.Repo.Name, github.CreateCheckRunOptions{
			Name:    ws.Name,
			HeadSHA: *o.checkSuite.CheckSuite.HeadSHA,
		})
		if err != nil {
			klog.Errorf("unable to create check run : %s", err.Error())
		}

		go webhookCheckRun()

		klog.Infof("created check run")
	}

	return nil
}

func (o *webhookCheckSuite) cloneRepo(token, path string) error {
	// If the directory already exists, check if it's at the right commit.  If
	// so, then we do nothing.
	if _, err := os.Stat(path); err == nil {
		klog.V(1).Infof("clone directory %q already exists, checking if it's at the right commit", path)

		// We use git rev-parse to see if our repo is at the right commit.
		out, err := runGitCmd(path, "rev-parse", "HEAD")
		if err != nil {
			klog.Warningf("will re-clone repo, could not determine if it was at correct commit: %s", err.Error())
			return o.forceClone(ev, token, path)
		}
		currCommit := strings.Trim(out, "\n")

		if currCommit == *ev.CheckSuite.HeadSHA {
			// Do nothing: we're still at the current commit
			return nil
		}

		klog.V(1).Infof("repo was already cloned but is not at correct commit, wanted %q got %q", *ev.CheckSuite.HeadSHA, currCommit)
		// We'll fall through to force-clone.
	}

	return o.forceClone(ev, token, path)
}

func (o *webhookCheckSuite) forceClone(token, path string) error {
	// Insert access token into clone url
	u, err := url.Parse(*ev.Repo.CloneURL)
	if err != nil {
		return fmt.Errorf("unable to parse clone url: %s: %w", *ev.Repo.CloneURL, err)
	}
	u.User = url.UserPassword("x-access-token", token)

	// TODO: use url.Redacted to redact token in logging, errors

	if err := os.RemoveAll(path); err != nil {
		return errors.Wrapf(err, "deleting dir %q before cloning", path)
	}

	// Create the directory and parents if necessary.
	if err := os.MkdirAll(path, 0700); err != nil {
		return fmt.Errorf("unable to make directory for repo: %w", err)
	}

	args := []string{"clone", "--branch", *ev.CheckSuite.HeadBranch, "--depth=1", "--single-branch", u.String(), path}
	if _, err := runGitCmd(path, args...); err != nil {
		return err
	}

	return nil
}

func runGitCmd(path string, args ...string) (string, error) {
	cmd := exec.Command("git", args...) // nolint: gosec
	cmd.Dir = path

	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("unable to run git command: %s: %w", args, err)
	}
	return string(out), nil
}

func filterWorkspaces(list *v1alpha1.WorkspaceList, url, path string) *v1alpha1.WorkspaceList {
	newList := v1alpha1.WorkspaceList{}

	for _, ws := range list.Items {
		// Ignore workspaces connected to a different repo
		if ws.Spec.VCS.Repository != url {
			continue
		}
		if _, err := os.Stat(filepath.Join(path, ws.Spec.VCS.WorkingDir)); err != nil {
			// Ignore workspaces connected to a non-existant sub-dir
			continue
		}
		newList.Items = append(newList.Items, ws)
	}

	return &newList
}
