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
func (a *app) handleEvent(event interface{}, action string, client checksClient) (result string, id int64, err error) {
	switch ev := event.(type) {
	case *github.CheckSuiteEvent:
		id = ev.GetCheckSuite().GetID()
		result, err = a.handleCheckSuiteEvent(ev, action)
	case *github.CheckRunEvent:
		id = ev.GetCheckRun().GetID()
		result, err = a.handleCheckRunEvent(ev, action)
	case *github.PullRequestEvent:
		id = ev.GetPullRequest().GetID()
		result, err = a.handlePullRequestEvent(ev, action, client)
	default:
		result = "ignored"
	}
	return result, id, err
}

// +kubebuilder:rbac:groups=etok.dev,resources=checksuites,verbs=create

// Handle incoming check suite events, and create corresponding k8s resources
func (a *app) handleCheckSuiteEvent(ev *github.CheckSuiteEvent, action string) (string, error) {
	if action != "requested" && action != "rerequested" {
		return "ignored", nil
	}

	if len(ev.CheckSuite.PullRequests) == 0 {
		return "no pulls found", nil
	}

	ctx := context.Background()

	suite := builders.CheckSuiteFromEvent(ev).Build()

	if action == "requested" {
		// Create CheckSuite resource
		if err := a.Create(ctx, suite); err != nil {
			return "", fmt.Errorf("unable to create kubernetes resource: %w", err)
		}
		return fmt.Sprintf("created check suite kubernetes resource: %s", klog.KObj(suite)), nil
	}

	if action == "rerequested" {
		// Increment rerequested counter
		if err := a.Get(ctx, runtimeclient.ObjectKeyFromObject(suite), suite); err != nil {
			return "", fmt.Errorf("unable to get check suite kubernetes resource: %w", err)
		}
		suite.Spec.Rerequests++
		err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			return a.Client.Update(ctx, suite)
		})
		if err != nil {
			return "", fmt.Errorf("unable to update check suite kubernetes resource: %w", err)
		}
		return fmt.Sprintf("updated check suite kubernetes resource: %s", klog.KObj(suite)), nil
	}

	// Should never reach here
	return "ignored", nil
}

// +kubebuilder:rbac:groups=etok.dev,resources=checkruns,verbs=get;update

// Handle incoming check run events, updating k8s resources accordingly
func (a *app) handleCheckRunEvent(ev *github.CheckRunEvent, action string) (string, error) {
	// Extract namespace/name of Check from the external ID field
	parts := strings.Split(ev.CheckRun.GetExternalID(), "/")
	if len(parts) != 2 {
		return "", fmt.Errorf("malformed external ID: %s", ev.CheckRun.GetExternalID())
	}

	// Update CheckRun resource with new event

	check := &v1alpha1.CheckRun{}
	checkKey := types.NamespacedName{Namespace: parts[0], Name: parts[1]}
	if err := a.Client.Get(context.Background(), checkKey, check); err != nil {
		return "", fmt.Errorf("unable to retrieve check run kubernetes resource: %w", err)
	}

	checkEvent := &v1alpha1.CheckRunEvent{Received: metav1.Now()}
	switch action {
	case "created":
		checkEvent.Created = &v1alpha1.CheckRunCreatedEvent{ID: ev.CheckRun.GetID()}
	case "rerequested":
		checkEvent.Rerequested = &v1alpha1.CheckRunRerequestedEvent{}
	case "requested_action":
		checkEvent.RequestedAction = &v1alpha1.CheckRunRequestedActionEvent{Action: ev.GetRequestedAction().Identifier}
	case "completed":
		checkEvent.Completed = &v1alpha1.CheckRunCompletedEvent{}
	default:
		return "ignored", nil
	}

	check.Status.Events = append(check.Status.Events, checkEvent)

	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		return a.Client.Status().Update(context.Background(), check)
	})
	if err != nil {
		return "", fmt.Errorf("unable to add event to check run kubernetes resource: %w", err)
	}

	return fmt.Sprintf("added %s event to check run resource: %s", action, klog.KObj(check)), nil
}

// +kubebuilder:rbac:groups=etok.dev,resources=checksuites,verbs=create

// Handle incoming pull request events, and create/update corresponding k8s
// resources
func (a *app) handlePullRequestEvent(ev *github.PullRequestEvent, action string, gclient checksClient) (string, error) {
	if action != "opened" {
		return "ignored", nil
	}

	results, _, err := gclient.ListCheckSuitesForRef(context.Background(), ev.Repo.Owner.GetLogin(), ev.Repo.GetName(), ev.PullRequest.Head.GetRef(), nil)
	if err != nil {
		return "", fmt.Errorf("unable to find check suite for pull: %w", err)
	}

	if results.GetTotal() == 0 {
		return "", fmt.Errorf("no check suites associated with pull")
	}

	// Impossible to have more than one check suite for a ref, no?
	suite := results.CheckSuites[0]

	// Check if k8s resource already exists
	resource := builders.CheckSuite(suite.GetID()).Build()
	if err := a.Get(context.Background(), runtimeclient.ObjectKeyFromObject(resource), resource); err != nil {
		if errors.IsNotFound(err) {
			// Create k8s resource
			bldr := builders.CheckSuiteFromObj(suite)
			bldr = bldr.InstallID(ev.GetInstallation().GetID())
			// CloneURL is missing in the suite obj, so retrieve from pull event
			// instead
			bldr = bldr.CloneURL(ev.GetRepo().GetCloneURL())
			resource = bldr.Build()
			if err := a.Create(context.Background(), resource); err != nil {
				return "", fmt.Errorf("unable to create check suite kubernetes resource: %w", err)
			}
			return fmt.Sprintf("created check suite kubernetes resource: %s", klog.KObj(resource)), nil
		}
		return "", fmt.Errorf("unable to retrieve check suite kubernetes resource: %w", err)
	}

	return "check suite kubernetes resource already exists", nil
}
