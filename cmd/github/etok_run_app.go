package github

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/google/go-github/v31/github"
	"github.com/leg100/etok/api/etok.dev/v1alpha1"
	"github.com/leg100/etok/pkg/client"
	"github.com/leg100/etok/pkg/controllers"
	"github.com/leg100/etok/pkg/launcher"
	"github.com/leg100/etok/pkg/logstreamer"
	"github.com/leg100/etok/pkg/util"
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
	// Set func to use for streaming logs from pod - exposed here for testing
	// purposes
	getLogsFunc logstreamer.GetLogsFunc
}

func newEtokRunApp(kClient *client.Client, opts etokAppOptions) *etokRunApp {
	return &etokRunApp{
		etokAppOptions: opts,
		kClient:        kClient,
		repoManager:    newRepoManager(opts.cloneDir),
	}
}

func (o *etokRunApp) handleEvent(client *GithubClient, event interface{}) error {
	launcherOptsList, err := o.createLauncherOptsList(client, event)
	if err != nil {
		return err
	}

	// Actually create each Run resource
	for _, opts := range launcherOptsList {
		if err := launcher.NewLauncher(opts).Launch(context.Background()); err != nil {
			// Failed to create resource. Ensure github check run is created
			// with error. (If it had the resource had been created, the run
			// monitor would have picked it up, and then had been responsible
			// for creating and updating its check run).
			client.send(r)
		}
	}

	return nil
}

// For a github event create a list of etok runs. The token refresher is
// required to clone repo from github.
func (o *etokRunApp) createLauncherOptsList(refresher tokenRefresher, event interface{}) ([]*launcher.LauncherOptions, error) {
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
				*ev.Repo.CloneURL,
				*ev.CheckRun.CheckSuite.HeadBranch,
				*ev.CheckRun.CheckSuite.HeadSHA,
				*ev.Repo.Owner.Login,
				*ev.Repo.Name, refresher)
			if err != nil {
				return nil, err
			}

			command := "plan"
			if ev.RequestedAction != nil && ev.RequestedAction.Identifier == "apply" {
				command = "apply"
			}

			klog.InfoS("check run event", "id", ev.CheckRun.GetID(), "command", command)

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

	etokRun, err := newEtokRun(a.kClient, command, originalRun, ws, repo, a.etokAppOptions)
	if err != nil {
		klog.Errorf("unable to create etok run: %s", err.Error())
		return nil, err
	}

	return etokRun, nil
}

// Create etok runs for each workspace 'connected' to the repo.
func (a *etokRunApp) createRuns(refresher tokenRefresher, repo *repo) ([]*launcher.LauncherOptions, error) {
	connected, err := getConnectedWorkspaces(a.kClient, repo.url)
	if err != nil {
		return nil, err
	}
	if len(connected.Items) == 0 {
		// No connected workspaces found
		return nil, nil
	}

	// Create check-run for each connected workspace
	launcherOptsList := []*launcher.LauncherOptions{}
	for _, ws := range connected.Items {
		// Get full path to workspace's working directory
		path := repo.workspacePath(&ws)

		// Skip workspaces with a non-existent working dir
		if _, err := os.Stat(path); os.IsNotExist(err) {
			klog.Warningf("skipping workspace %s with non-existent working directory: %s", klog.KObj(&ws), ws.Spec.VCS.WorkingDir)
			continue
		}

		opts, err := newLauncherOpts(a.kClient, "plan", "", &ws, repo, a.etokAppOptions)
		if err != nil {
			klog.Errorf("unable to create an etok run: %s", err.Error())
			continue
		}

		launcherOptsList = append(launcherOptsList, opts)
	}

	return launcherOptsList, nil
}

// Constructor for an etok run obj
func newLauncherOpts(kClient *client.Client, command, previous string, workspace *v1alpha1.Workspace, repo *repo, appOpts etokAppOptions) (*launcher.LauncherOptions, error) {
	id := fmt.Sprintf("run-%s", util.GenerateRandomString(5))

	args, err := launcherArgs(id, command, previous)
	if err != nil {
		return nil, err
	}

	opts := &launcher.LauncherOptions{
		Client:      kClient,
		Workspace:   workspace.Name,
		Namespace:   workspace.Namespace,
		DisableTTY:  true,
		Command:     "sh",
		Args:        args,
		Path:        repo.workspacePath(workspace),
		RunName:     id,
		Status:      &appOpts.runStatus,
		GetLogsFunc: appOpts.getLogsFunc,
	}

	return opts, nil
}

func launcherArgs(id, command, previous string) ([]string, error) {
	script := new(bytes.Buffer)

	// Default is to create a new plan file with a filename the same as the etok
	// run ID
	planPath := filepath.Join(controllers.PlansMountPath, id)
	if command == "apply" {
		// Apply uses the plan file from the previous run
		planPath = filepath.Join(controllers.PlansMountPath, previous)
	}

	if err := generateEtokRunScript(script, planPath, command); err != nil {
		klog.Errorf("unable to generate check run script: %s", err.Error())
		return nil, err
	}

	return []string{script.String()}, nil
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
