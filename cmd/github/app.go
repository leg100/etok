package github

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/google/go-github/v31/github"
	"github.com/leg100/etok/api/etok.dev/v1alpha1"
	"github.com/leg100/etok/pkg/archive"
	"github.com/leg100/etok/pkg/builders"
	"github.com/leg100/etok/pkg/client"
	"github.com/leg100/etok/pkg/controllers"
	"github.com/leg100/etok/pkg/util"
	"k8s.io/klog/v2"
)

type app struct {
	kclient *client.Client

	appOptions

	*repoManager
}

type appOptions struct {
	// Path to directory to which git repositories are cloned
	cloneDir        string
	stripRefreshing bool
	// Override run state - for testing purposes
	runStatus v1alpha1.RunStatus
}

func newApp(kclient *client.Client, opts appOptions) *app {
	return &app{
		appOptions:  opts,
		kclient:     kclient,
		repoManager: newRepoManager(opts.cloneDir),
	}
}

// Handle incoming github events, creating check runs and etok run resources
// accordingly.
func (a *app) handleEvent(event interface{}, client githubClientInterface) error {
	switch ev := event.(type) {
	case *github.CheckSuiteEvent:
		switch *ev.Action {
		case "requested", "rerequested":
			// Number of runs created
			var created int

			klog.InfoS("received check suite event", "id", ev.CheckSuite.GetID(), "action", *ev.Action)
			defer func() {
				klog.InfoS("finished handling check suite event", "id", ev.CheckSuite.GetID(), "check_runs_created", created)
			}()

			repo, err := a.repoManager.clone(
				*ev.Repo.CloneURL,
				*ev.CheckSuite.HeadBranch,
				*ev.CheckSuite.HeadSHA,
				*ev.Repo.Owner.Login,
				*ev.Repo.Name, client)
			if err != nil {
				return err
			}

			connected, err := getConnectedWorkspaces(a.kclient, repo.url)
			if err != nil {
				return err
			}
			if len(connected.Items) == 0 {
				// No connected workspaces found
				return nil
			}

			// Create check run for each connected workspace
			for _, ws := range connected.Items {
				// Skip workspaces with a non-existent working dir
				if _, err := os.Stat(filepath.Join(repo.path, ws.Spec.VCS.WorkingDir)); os.IsNotExist(err) {
					klog.Warningf("skipping workspace %s with non-existent working directory: %s", klog.KObj(&ws), ws.Spec.VCS.WorkingDir)
					continue
				}

				client.send(&checkRun{
					id:           fmt.Sprintf("run-%s", util.GenerateRandomString(5)),
					namespace:    ws.Namespace,
					workspace:    ws.Name,
					sha:          *ev.CheckSuite.HeadSHA,
					owner:        *ev.Repo.Owner.Login,
					repo:         *ev.Repo.Name,
					command:      "plan",
					maxFieldSize: defaultMaxFieldSize,
					iteration:    1,
				})

				created++
			}

			return nil
		default:
			klog.InfoS("ignoring check suite event", "id", ev.CheckSuite.GetID(), "action", *ev.Action)
		}
	case *github.CheckRunEvent:
		switch *ev.Action {
		case "created", "rerequested", "requested_action":

			klog.InfoS("received check run event", "id", ev.CheckRun.GetID(), "check_suite_id", ev.CheckRun.CheckSuite.GetID(), "action", *ev.Action)
			defer func() {
				klog.InfoS("finished handling check event", "id", ev.CheckRun.GetID(), "check_suite_id", ev.CheckRun.CheckSuite.GetID())
			}()

			// Extract metadata from the external ID field
			metadata := newCheckRunMetadata(ev.CheckRun.ExternalID)

			if ev.RequestedAction != nil {
				// Override command with whatever the user has requested
				metadata.Command = ev.RequestedAction.Identifier
			}

			repo, err := a.repoManager.clone(
				*ev.Repo.CloneURL,
				*ev.CheckRun.CheckSuite.HeadBranch,
				*ev.CheckRun.CheckSuite.HeadSHA,
				*ev.Repo.Owner.Login,
				*ev.Repo.Name, client)
			if err != nil {
				return err
			}

			ws, err := a.kclient.WorkspacesClient(metadata.Namespace).Get(context.Background(), metadata.Workspace, metav1.GetOptions{})
			if err != nil {
				return err
			}

			switch *ev.Action {
			case "rerequested", "requested_action":
				client.send(&checkRun{
					id:           fmt.Sprintf("run-%s", util.GenerateRandomString(5)),
					namespace:    ws.Namespace,
					workspace:    ws.Name,
					sha:          *ev.CheckRun.CheckSuite.HeadSHA,
					owner:        *ev.Repo.Owner.Login,
					repo:         *ev.Repo.Name,
					command:      metadata.Command,
					previous:     metadata.Current,
					maxFieldSize: defaultMaxFieldSize,
				})
			case "created":
				// Check run has been created. Only now can we create a Run
				// resource because we need to label it with the check run ID.

				script := runScript(metadata.Current, metadata.Command, metadata.Previous)

				bldr := builders.Run(ws.Namespace, metadata.Current, ws.Name, "sh", script)

				// For testing purposes seed status
				bldr.SetStatus(a.runStatus)

				bldr.SetLabel(githubAppInstallIDLabelName, strconv.Itoa(int(client.getInstallID())))
				bldr.SetLabel(checkRunIDLabelName, strconv.Itoa(int(ev.CheckRun.GetID())))
				bldr.SetLabel(checkRunOwnerLabelName, *ev.Repo.Owner.Login)
				bldr.SetLabel(checkRunRepoLabelName, *ev.Repo.Name)
				bldr.SetLabel(checkRunSHALabelName, *ev.CheckRun.CheckSuite.HeadSHA)
				bldr.SetLabel(checkRunCommandLabelName, metadata.Command)

				configMap, err := archive.ConfigMap(ws.Namespace, metadata.Current, filepath.Join(repo.path, ws.Spec.VCS.WorkingDir), repo.path)
				if err != nil {
					return err
				}

				r := bldr.Build()

				// Create Run/ConfigMap resources in k8s
				if err := createRunAndArchive(a.kclient, r, configMap); err != nil {
					// Failed to create resources. Update checkrun reporting
					// failure to user.
					run, err := newRunFromResource(r, err)
					if err != nil {
						return err
					}

					// Must provide check run ID otherwise the client will
					// create a brand new check run
					run.checkRunId = ev.CheckRun.ID

					client.send(run)
				}
			}

			return nil
		default:
			klog.InfoS("ignoring check run event", "check_suite_id", ev.CheckRun.CheckSuite.GetID(), "id", ev.CheckRun.GetID(), "action", *ev.Action)
		}
	default:
		klog.Infof("ignoring event: %T", event)
	}

	return nil
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
		return "", "", fmt.Errorf("malformed object ref: '%s'", orig)
	}
	return parts[0], parts[1], nil
}

// Create Run and ConfigMap resources in k8s
func createRunAndArchive(client *client.Client, run *v1alpha1.Run, archive *corev1.ConfigMap) error {
	_, err := client.RunsClient(run.Namespace).Create(context.Background(), run, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	_, err = client.ConfigMapsClient(run.Namespace).Create(context.Background(), archive, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	return nil
}

func runScript(id, command, previous string) string {
	script := new(bytes.Buffer)

	// Default is to create a new plan file with a filename the same as the etok
	// run ID
	planPath := filepath.Join(controllers.PlansMountPath, id)
	if command == "apply" {
		// Apply uses the plan file from the previous run
		planPath = filepath.Join(controllers.PlansMountPath, previous)
	}

	if err := generateEtokRunScript(script, planPath, command); err != nil {
		panic("unable to generate check run script: " + err.Error())
	}

	return script.String()
}
