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

	*repoManager
}

type etokAppOptions struct {
	// Path to directory to which git repositories are cloned
	cloneDir        string
	stripRefreshing bool
	// Override run state - for testing purposes
	runStatus v1alpha1.RunStatus
}

func newEtokRunApp(kClient *client.Client, opts etokAppOptions) *etokRunApp {
	return &etokRunApp{
		etokAppOptions: opts,
		kClient:        kClient,
		repoManager:    newRepoManager(opts.cloneDir),
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

// For a github event create a list of etok runs. The token refresher is
// required to clone repo from github.
func (o *etokRunApp) createEtokRuns(refresher tokenRefresher, event interface{}) ([]*etokRun, error) {
	switch ev := event.(type) {
	case *github.CheckSuiteEvent:
		klog.InfoS("received check suite event", "id", ev.CheckSuite.GetID(), "action", *ev.Action)

		switch *ev.Action {
		case "requested", "rerequested":
			repo, err := o.repoManager.clone(
				*ev.Repo.CloneURL,
				*ev.CheckSuite.HeadBranch,
				*ev.CheckSuite.HeadSHA,
				*ev.Repo.Owner.Login,
				*ev.Repo.Name, refresher)
			if err != nil {
				return nil, err
			}

			runs, err := o.createRuns(refresher, repo)
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

			repo, err := o.repoManager.clone(
				*ev.CheckRun.CheckSuite.Repository.CloneURL,
				*ev.CheckRun.CheckSuite.HeadBranch,
				*ev.CheckRun.CheckSuite.HeadCommit.SHA,
				*ev.CheckRun.CheckSuite.Repository.Owner.Login,
				*ev.CheckRun.CheckSuite.Repository.Name, refresher)
			if err != nil {
				return nil, err
			}

			command := "plan"
			if ev.RequestedAction != nil && ev.RequestedAction.Identifier == "apply" {
				command = "apply"
			}

			run, err := o.rerun(refresher, repo, *ev.CheckRun.ExternalID, command)
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
func (a *etokRunApp) rerun(refresher tokenRefresher, repo *repo, previous, command string) (*etokRun, error) {
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

	// Skip workspaces with a non-existent working dir
	if _, err := os.Stat(repo.workspacePath(ws)); os.IsNotExist(err) {
		klog.Warningf("skipping workspace %s with non-existent working directory: %s", klog.KObj(ws), ws.Spec.VCS.WorkingDir)
		return nil, err
	}

	etokRun, err := newEtokRun(a.kClient, command, "", ws, repo, a.etokAppOptions)
	if err != nil {
		klog.Errorf("unable to create etok run: %s", err.Error())
		return nil, err
	}

	return etokRun, nil
}

// Create etok runs for each workspace 'connected' to the repo.
func (a *etokRunApp) createRuns(refresher tokenRefresher, repo *repo) ([]*etokRun, error) {
	connected, err := getConnectedWorkspaces(a.kClient, repo.url)
	if err != nil {
		return nil, err
	}
	if len(connected.Items) == 0 {
		// No connected workspaces found
		return nil, nil
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
