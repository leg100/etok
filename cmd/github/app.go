package github

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"

	"github.com/google/go-github/v31/github"
	"github.com/leg100/etok/api/etok.dev/v1alpha1"
	"k8s.io/klog/v2"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type app struct {
	runtimeclient.Client

	appOptions
}

type appOptions struct {
	// Path to directory to which git repositories are cloned
	cloneDir        string
	stripRefreshing bool
	// Override run state - for testing purposes
	runStatus v1alpha1.RunStatus
}

func newApp(client runtimeclient.Client, opts appOptions) *app {
	return &app{
		Client:     client,
		appOptions: opts,
	}
}

// Handle incoming github events, creating/updating k8s resources accordingly
func (a *app) handleEvent(event interface{}, mgr installsManager) error {
	switch ev := event.(type) {
	case *github.CheckSuiteEvent:
		switch *ev.Action {
		// Either of these events leads to the creation of a new CheckSuite
		// resource
		case "requested", "rerequested":
			klog.InfoS("received check suite event", "id", ev.CheckSuite.GetID(), "action", *ev.Action)
			defer func() {
				klog.InfoS("finished handling check suite event", "id", ev.CheckSuite.GetID())
			}()

			// Create check run for each connected workspace
			suite := &v1alpha1.CheckSuite{
				ObjectMeta: metav1.ObjectMeta{
					Name: strconv.FormatInt(ev.CheckSuite.GetID(), 10),
				},
				Spec: v1alpha1.CheckSuiteSpec{
					InstallID: ev.GetInstallation().GetID(),
					SHA:       ev.CheckSuite.GetHeadSHA(),
					Owner:     ev.Repo.Owner.GetLogin(),
					Repo:      ev.Repo.GetName(),
				},
			}

			// Create CheckSuite resource
			if err := a.Create(context.Background(), suite); err != nil {
				return err
			}

			return nil
		default:
			klog.InfoS("ignoring check suite event", "id", ev.CheckSuite.GetID(), "action", *ev.Action)
		}
	case *github.CheckRunEvent:
		klog.InfoS("received check run event", "id", ev.CheckRun.GetID(), "check_suite_id", ev.CheckRun.CheckSuite.GetID(), "action", *ev.Action)
		defer func() {
			klog.InfoS("finished handling check event", "id", ev.CheckRun.GetID(), "check_suite_id", ev.CheckRun.CheckSuite.GetID())
		}()

		// Extract namespace/name of Check from the external ID field
		parts := strings.Split(ev.CheckRun.GetExternalID(), "/")
		if len(parts) != 2 {
			return fmt.Errorf("malformed external ID: %s", ev.CheckRun.GetExternalID())
		}

		// Update CheckRun resource with new event

		check := &v1alpha1.CheckRun{}
		checkKey := types.NamespacedName{Namespace: parts[0], Name: parts[1]}
		if err := a.Client.Get(context.Background(), checkKey, check); err != nil {
			return err
		}

		checkEvent := &v1alpha1.CheckRunEvent{Received: metav1.Now()}
		switch ev.GetAction() {
		case "created":
			checkEvent.Created = &v1alpha1.CheckRunCreatedEvent{ID: ev.CheckRun.GetID()}
		case "rerequested":
			checkEvent.Rerequested = &v1alpha1.CheckRunRerequestedEvent{}
		case "requested_action":
			checkEvent.RequestedAction = &v1alpha1.CheckRunRequestedActionEvent{Action: ev.GetRequestedAction().Identifier}
		case "completed":
			checkEvent.Completed = &v1alpha1.CheckRunCompletedEvent{}
		default:
			return fmt.Errorf("unexpected check run action received: %s", ev.GetAction())
		}

		check.Status.Events = append(check.Status.Events, checkEvent)

		err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			return a.Client.Status().Update(context.Background(), check)
		})
		if err != nil {
			return err
		}

		return nil
	default:
		klog.Infof("ignoring event: %T", event)
	}

	return nil
}

// isMergeable determines if a check run is 'mergeable': all of its PRs must be
// mergeable, or it must have zero PRs. Otherwise it is not deemed mergeable.
func isMergeable(checkRun *github.CheckRun) bool {
	for _, pr := range checkRun.PullRequests {
		state := pr.GetMergeableState()
		if state != "clean" && state != "unstable" && state != "has_hooks" {
			return false
		}
	}
	return true
}
