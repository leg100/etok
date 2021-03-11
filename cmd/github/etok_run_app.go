package github

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/google/go-github/v31/github"
	"github.com/leg100/etok/api/etok.dev/v1alpha1"
	"github.com/leg100/etok/pkg/client"
	"k8s.io/klog/v2"
)

type etokRunApp struct {
	kClient *client.Client

	etokAppOptions
}

type etokAppOptions struct {
	// Path to directory to which git repositories are cloned
	cloneDir        string
	stripRefreshing bool
}

func newEtokRunApp(kClient *client.Client, opts etokAppOptions) *etokRunApp {
	return &etokRunApp{
		etokAppOptions: opts,
		kClient:        kClient,
	}
}

func (o *etokRunApp) handleEvent(client *GithubClient, event interface{}) error {
	runs, err := o.createEtokRuns(client, event)
	if err != nil {
		return err
	}

	for _, r := range runs {
		go monitor(client, r, time.Second)
	}

	return nil
}

func (o *etokRunApp) createEtokRuns(client *GithubClient, event interface{}) ([]*etokRun, error) {
	switch ev := event.(type) {
	case *github.CheckSuiteEvent:
		klog.InfoS("received check suite event", "id", ev.CheckSuite.GetID(), "action", *ev.Action)

		switch *ev.Action {
		case "requested", "rerequested":
			runs, err := o.createRuns(client, o.newRepoFromCheckSuiteEvent(ev))
			if err != nil {
				return nil, err
			}

			klog.InfoS("finished handling check suite event", "id", ev.CheckSuite.GetID(), "check_runs_created", len(runs))
			return runs, nil
		}
	case *github.CheckRunEvent:
		klog.InfoS("received check run event", "check_suite_id", ev.CheckRun.CheckSuite.GetID(), "id", ev.CheckRun.GetID(), "action", *ev.Action)

		switch *ev.Action {
		case "rerequested", "requested_action":
			// User has requested that a check-run be re-run. We need to lookup
			// the original connected etok run and workspace, so that we know
			// what to re-run (or apply)

			repo := o.newRepoFromCheckRunEvent(ev)

			command := "plan"
			if ev.RequestedAction != nil && ev.RequestedAction.Identifier == "apply" {
				command = "apply"
			}

			run, err := o.rerun(client, repo, *ev.CheckRun.ExternalID, command)
			if err != nil {
				return nil, err
			}

			return []*etokRun{run}, nil
		}
	default:
		klog.Infof("ignoring event: %T", event)
	}

	return nil, nil
}

// Re-run, or apply, a previous plan.
func (a *etokRunApp) rerun(ghClient *GithubClient, repo *repo, previous, command string) (*etokRun, error) {
	namespace, originalRun, err := splitObjectRef(previous)
	if err != nil {
		return nil, err
	}

	run, err := a.kClient.RunsClient(namespace).Get(context.Background(), originalRun, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	ws, err := a.kClient.WorkspacesClient(namespace).Get(context.Background(), run.Workspace, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	// Ensure we have a cloned repo on disk
	if err := repo.ensureCloned(ghClient); err != nil {
		return nil, err
	}

	// Skip workspaces with a non-existent working dir
	if _, err := os.Stat(repo.workspacePath(ws)); os.IsNotExist(err) {
		klog.Warningf("skipping workspace %s with non-existent working directory: %s", klog.KObj(ws), ws.Spec.VCS.WorkingDir)
		return nil, err
	}

	etokRun, err := newEtokRun(a.kClient, "plan", "", ws, repo, a.etokAppOptions)
	if err != nil {
		klog.Errorf("unable to create etok run: %s", err.Error())
		return nil, err
	}

	return etokRun, nil
}

// Create a slice of etok runs for the given repo.
func (a *etokRunApp) createRuns(ghClient *GithubClient, repo *repo) ([]*etokRun, error) {
	// Find connected workspaces and create a check-run for each one.
	connected, err := getConnectedWorkspaces(a.kClient, repo.url)
	if err != nil {
		return nil, err
	}
	if len(connected.Items) == 0 {
		// No connected workspaces found
		return nil, nil
	}

	// Ensure we have a cloned repo on disk
	if err := repo.ensureCloned(ghClient); err != nil {
		return nil, err
	}

	// Create check-run for each connected workspace
	etokRuns := []*etokRun{}
	for _, ws := range connected.Items {
		// Get full path to workspace's working directory
		path := repo.workspacePath(&ws)

		// Skip workspaces with a non-existent working dir
		if _, err := os.Stat(path); os.IsNotExist(err) {
			klog.Warningf("skipping workspace %s with non-existent working directory: %s", klog.KObj(&ws), ws.Spec.VCS.WorkingDir)
			continue
		}

		run, err := newEtokRun(a.kClient, "plan", "", &ws, repo, a.etokAppOptions)
		if err != nil {
			klog.Errorf("unable to create an etok run: %s", err.Error())
			continue
		}

		etokRuns = append(etokRuns, run)
	}

	return etokRuns, nil
}

func (o *etokRunApp) newRepoFromCheckSuiteEvent(ev *github.CheckSuiteEvent) *repo {
	return &repo{
		parentDir: o.cloneDir,
		url:       *ev.CheckSuite.GetRepository().CloneURL,
		branch:    *ev.CheckSuite.HeadBranch,
		sha:       *ev.CheckSuite.GetHeadCommit().SHA,
		owner:     *ev.CheckSuite.GetRepository().GetOwner().Login,
		name:      *ev.CheckSuite.GetRepository().Name,
	}
}

func (o *etokRunApp) newRepoFromCheckRunEvent(ev *github.CheckRunEvent) *repo {
	return &repo{
		parentDir: o.cloneDir,
		url:       *ev.GetRepo().CloneURL,
		branch:    *ev.CheckRun.CheckSuite.HeadBranch,
		sha:       *ev.GetCheckRun().HeadSHA,
		owner:     *ev.GetRepo().GetOwner().Login,
		name:      *ev.GetRepo().GetOwner().Name,
	}
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
		if !ws.IsConnected(url) {
			klog.V(2).Infof("Skipping unconnected workspace %s", klog.KObj(&ws))
			continue
		}
		connected.Items = append(connected.Items, ws)
	}

	return &connected, nil
}

func splitObjectRef(orig string) (string, string, error) {
	parts := strings.Split(orig, "/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("malformed object ref: %s", orig)
	}
	return parts[0], parts[1], nil
}
