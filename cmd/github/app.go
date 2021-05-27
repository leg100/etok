package github

import (
	"context"
	"fmt"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
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

	// getter permits the webhook server to retrieve github clients for
	// different installations
	getter clientGetter
}

func newApp(client runtimeclient.Client) *app {
	return &app{
		Client: client,
	}
}

// Handle incoming github events
func (a *app) handleEvent(ev event, clients githubClients) (result string, id int64, err error) {

	switch ev := ev.(type) {
	case *github.CheckSuiteEvent:
		id = ev.GetCheckSuite().GetID()
		result, err = a.handleCheckSuiteEvent(ev, ev.GetAction())
	case *github.CheckRunEvent:
		id = ev.GetCheckRun().GetID()
		result, err = a.handleCheckRunEvent(ev, ev.GetAction())
	case *github.PullRequestEvent:
		id = ev.GetPullRequest().GetID()
		result, err = a.handlePullRequestEvent(ev, ev.GetAction(), clients)
	case *github.PullRequestReviewEvent:
		id = ev.GetPullRequest().GetID()
		result, err = a.handlePullRequestReviewEvent(ev, ev.GetAction(), clients)
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

// Handle incoming pull request events. On every event action ensure there is a
// CheckSuite k8s resource, and update its mergeable status.
func (a *app) handlePullRequestEvent(ev *github.PullRequestEvent, action string, gclients githubClients) (string, error) {
	return a.updateCheckSuiteStatus(
		gclients,
		ev.GetRepo().GetOwner().GetLogin(),
		ev.GetRepo().GetName(),
		ev.GetPullRequest().GetHead().GetRef(),
		ev.GetRepo().GetCloneURL(),
		ev.GetInstallation().GetID(),
		ev.GetPullRequest().GetNumber(),
	)
}

// Handle incoming pull request review events. On every event action ensure
// there is a CheckSuite k8s resource, and update its mergeable status.
func (a *app) handlePullRequestReviewEvent(ev *github.PullRequestReviewEvent, action string, gclients githubClients) (string, error) {
	return a.updateCheckSuiteStatus(
		gclients,
		ev.GetRepo().GetOwner().GetLogin(),
		ev.GetRepo().GetName(),
		ev.GetPullRequest().GetHead().GetRef(),
		ev.GetRepo().GetCloneURL(),
		ev.GetInstallation().GetID(),
		ev.GetPullRequest().GetNumber(),
	)
}

func (a *app) updateCheckSuiteStatus(gclients githubClients, owner, repo, ref, cloneURL string, installID int64, pullNumber int) (string, error) {
	ctx := context.Background()

	suite, err := getSuiteFromRef(ctx, gclients.checks, owner, repo, ref)
	if err != nil {
		return "", fmt.Errorf("unable to find check suite for pull: %w", err)
	}

	// We're performing multiple steps so we'll have multiple results to return
	var results []string

	resource, created, err := a.ensureCheckSuiteResourceExists(ctx, suite, installID, cloneURL)
	if err != nil {
		return "", err
	}
	if created {
		results = append(results, fmt.Sprintf("created check suite kubernetes resource: %s", klog.KObj(resource)))
	}

	updated, err := a.updateMergeableStatus(ctx, gclients.pulls, resource, owner, repo, pullNumber)
	if err != nil {
		return "", err
	}
	if updated {
		results = append(results, fmt.Sprintf("mergeable status updated: %v", resource.Status.Mergeable))
	} else {
		results = append(results, fmt.Sprintf("mergeable status unchanged: %v", resource.Status.Mergeable))
	}

	return strings.Join(results, ", "), nil
}

func getSuiteFromRef(ctx context.Context, client checksClient, owner, repo, ref string) (*github.CheckSuite, error) {
	suites, _, err := client.ListCheckSuitesForRef(ctx, owner, repo, ref, nil)
	if err != nil {
		return nil, err
	}
	if suites.GetTotal() == 0 {
		return nil, fmt.Errorf("no check suites associated with pull")
	}

	// Impossible to have more than one check suite for a ref, no?
	return suites.CheckSuites[0], nil
}

// +kubebuilder:rbac:groups=etok.dev,resources=checksuites,verbs=get,create
func (a *app) ensureCheckSuiteResourceExists(ctx context.Context, obj *github.CheckSuite, installID int64, cloneURL string) (*v1alpha1.CheckSuite, bool, error) {
	resource := builders.CheckSuite(obj.GetID()).Build()
	err := a.Client.Get(ctx, runtimeclient.ObjectKeyFromObject(resource), resource)
	if err == nil {
		return resource, false, nil
	}
	if errors.IsNotFound(err) {
		// Create k8s resource
		resource := builders.CheckSuiteFromObj(obj).InstallID(installID).CloneURL(cloneURL).Build()
		if err := a.Client.Create(ctx, resource); err != nil {
			return nil, false, fmt.Errorf("unable to create check suite kubernetes resource: %w", err)
		}
		return resource, true, nil
	} else {
		return nil, false, fmt.Errorf("unable to retrieve check suite kubernetes resource: %w", err)
	}
}

// +kubebuilder:rbac:groups=etok.dev,resources=checksuites,verbs=get
// +kubebuilder:rbac:groups=etok.dev,resources=checksuites/status,verbs=update
//
// Update mergeable status if it has changed
func (a *app) updateMergeableStatus(ctx context.Context, pullsClient pullsClient, resource *v1alpha1.CheckSuite, owner, repo string, pullNumber int) (bool, error) {
	mergeable, err := isMergeable(pullsClient, owner, repo, pullNumber)
	if err != nil {
		return false, fmt.Errorf("unable to check mergeable status of pull: %w", err)
	}
	if resource.Status.Mergeable == mergeable {
		return false, nil
	}

	// Propagate new mergeable status to kubernetes resource
	err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		if err := a.Client.Get(context.Background(), runtimeclient.ObjectKeyFromObject(resource), resource); err != nil {
			return err
		}
		resource.Status.Mergeable = mergeable
		return a.Client.Status().Update(context.Background(), resource)
	})
	if err != nil {
		return false, fmt.Errorf("unable to update mergeable status of check suite kubernetes resource: %w", err)
	}
	return true, nil
}

// Check mergeable status:
//
// https://docs.github.com/en/rest/guides/getting-started-with-the-git-database-api#checking-mergeability-of-pull-requests
//
func isMergeable(client pullsClient, owner, repo string, number int) (bool, error) {
	err := wait.Poll(time.Second, 10*time.Second, func() (bool, error) {
		pull, _, err := client.Get(context.Background(), owner, repo, number)
		if err != nil {
			return false, fmt.Errorf("unable to retrieve pull: %w", err)
		}
		state := pull.GetMergeableState()
		if state != "clean" && state != "unstable" && state != "has_hooks" {
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		return false, nil
	}
	return true, nil
}
