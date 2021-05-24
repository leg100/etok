package github

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"

	"github.com/google/go-github/v31/github"
	"github.com/leg100/etok/api/etok.dev/v1alpha1"
	"github.com/leg100/etok/pkg/builders"
	"k8s.io/klog/v2"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type app struct {
	// K8s controller-runtime client
	runtimeclient.Client
}

func newApp(client runtimeclient.Client) *app {
	return &app{
		Client: client,
	}
}

// Handle incoming github events
func (a *app) handleEvent(event interface{}, client checksClient) error {
	switch ev := event.(type) {
	case *github.CheckSuiteEvent:
		return a.handleCheckSuiteEvent(ev)
	case *github.CheckRunEvent:
		return a.handleCheckRunEvent(ev)
	case *github.PullRequestEvent:
		return a.handlePullRequestEvent(ev, client)
	default:
		klog.Infof("ignoring event: %T", event)
	}

	return nil
}

// +kubebuilder:rbac:groups=etok.dev,resources=checksuites,verbs=create

// Handle incoming check suite events, and create corresponding k8s resources
func (a *app) handleCheckSuiteEvent(ev *github.CheckSuiteEvent) error {
	switch *ev.Action {
	// Either of these events leads to the creation of a new CheckSuite
	// resource
	case "requested", "rerequested":
		klog.InfoS("received check suite event", "id", ev.CheckSuite.GetID(), "action", *ev.Action)
		defer func() {
			klog.InfoS("finished handling check suite event", "id", ev.CheckSuite.GetID())
		}()

		if len(ev.CheckSuite.PullRequests) == 0 {
			// Ignore check suites unrelated to a pull
			return nil
		}

		// Create CheckSuite resource
		if err := a.Create(context.Background(), builders.CheckSuiteFromEvent(ev).Build()); err != nil {
			return err
		}

		return nil
	default:
		klog.InfoS("ignoring check suite event", "id", ev.CheckSuite.GetID(), "action", *ev.Action)
	}

	return nil
}

// +kubebuilder:rbac:groups=etok.dev,resources=checkruns,verbs=get;update

// Handle incoming check run events, updating k8s resources accordingly
func (a *app) handleCheckRunEvent(ev *github.CheckRunEvent) error {
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

	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		return a.Client.Status().Update(context.Background(), check)
	})
}

// +kubebuilder:rbac:groups=etok.dev,resources=checksuites,verbs=create

// Handle incoming pull request events, and create/update corresponding k8s
// resources
func (a *app) handlePullRequestEvent(ev *github.PullRequestEvent, gclient checksClient) error {
	switch *ev.Action {
	case "opened":
		klog.InfoS("received pull request event", "id", ev.PullRequest.GetID(), "action", ev.GetAction())

		// Lookup corresponding Check Suite ID in GH API
		results, _, err := gclient.ListCheckSuitesForRef(context.Background(), ev.Repo.Owner.GetLogin(), ev.Repo.GetName(), ev.PullRequest.Head.GetRef(), nil)
		if err != nil {
			return err
		}

		// No check suites associated with ref - isn't this impossible?
		if results.GetTotal() == 0 {
			return nil
		}

		// Impossible to have more than check suite for a ref, no?
		suite := results.CheckSuites[0]

		// Build the k8s resource
		bldr := builders.CheckSuiteFromObj(suite)
		bldr = bldr.InstallID(ev.GetInstallation().GetID())
		resource := bldr.Build()

		// Check if k8s resource already exists
		if err := a.Get(context.Background(), runtimeclient.ObjectKeyFromObject(resource), resource); err != nil {
			if errors.IsNotFound(err) {
				// Create k8s resource
				if err := a.Create(context.Background(), resource); err != nil {
					return err
				}
			} else {
				return err
			}
		}

		// See if k8s resource exists already. If so add a new
		// request/iteration. Otherwise create new resource.

		// Set PR info on k8s resource such as mergeable state. (How to get # of
		// approvers?)

		return nil
	default:
		klog.InfoS("ignoring check suite event", "id", ev.PullRequest.GetID(), "action", *ev.Action)
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
