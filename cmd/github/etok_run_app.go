package github

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/google/go-github/v31/github"
	"github.com/leg100/etok/api/etok.dev/v1alpha1"
	"github.com/leg100/etok/pkg/client"
	"github.com/leg100/etok/pkg/logstreamer"
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
	runArchives, err := o.constructRunArchives(client, event)
	if err != nil {
		return err
	}

	// Actually create Run/ConfigMap resources
	for _, ra := range runArchives {
		if err := ra.create(o.kClient); err != nil {
			// Failed to create resources. Create a checkrun reporting failure
			// to user.
			run, err := newRunFromResource(ra.run)
			if err != nil {
				return err
			}
			client.send(run)
		}
	}

	return nil
}

// Construct a list of Run/ConfigMaps from a given event. The token refresher is
// required to clone repo from github.
func (o *etokRunApp) constructRunArchives(refresher tokenRefresher, event interface{}) ([]*runArchive, error) {
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

			connected, err := getConnectedWorkspaces(o.kClient, repo.url)
			if err != nil {
				return nil, err
			}
			if len(connected.Items) == 0 {
				// No connected workspaces found
				return nil, nil
			}

			// Create Run/ConfigMap for each connected workspace
			var runArchives []*runArchive
			for _, ws := range connected.Items {
				// Skip workspaces with a non-existent working dir
				if _, err := os.Stat(filepath.Join(repo.path, ws.Spec.VCS.WorkingDir)); os.IsNotExist(err) {
					klog.Warningf("skipping workspace %s with non-existent working directory: %s", klog.KObj(&ws), ws.Spec.VCS.WorkingDir)
					continue
				}

				ra, err := newRunArchive(&ws, "plan", "", repo.path, &o.etokAppOptions.runStatus)
				if err != nil {
					return nil, err
				}

				runArchives = append(runArchives, ra)
			}

			klog.InfoS("finished handling check suite event", "id", ev.CheckSuite.GetID(), "check_runs_created", len(runArchives))
			return runArchives, nil
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

			// Retrieve original run's name and namespace from the external ID
			// field
			namespace, originalRun, err := splitObjectRef(*ev.CheckRun.ExternalID)
			if err != nil {
				return nil, err
			}

			run, err := o.kClient.RunsClient(namespace).Get(context.Background(), originalRun, metav1.GetOptions{})
			if err != nil {
				return nil, err
			}

			ws, err := o.kClient.WorkspacesClient(namespace).Get(context.Background(), run.Workspace, metav1.GetOptions{})
			if err != nil {
				return nil, err
			}

			// Skip if working directory doesn't exist in repo
			if _, err := os.Stat(repo.workspacePath(ws)); os.IsNotExist(err) {
				klog.Warningf("skipping workspace %s with non-existent working directory: %s", klog.KObj(ws), ws.Spec.VCS.WorkingDir)
				return nil, err
			}

			ra, err := newRunArchive(ws, command, originalRun, repo.path, &o.etokAppOptions.runStatus)
			if err != nil {
				return nil, err
			}

			return []*runArchive{ra}, nil
		}
	default:
		klog.Infof("ignoring event: %T", event)
	}

	return nil, nil
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
