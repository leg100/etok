package github

import (
	"bytes"
	"context"
	"fmt"
	"regexp"

	"github.com/google/go-github/v31/github"
	"github.com/leg100/etok/api/etok.dev/v1alpha1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

type checkRunClient interface {
	CreateCheckRun(context.Context, string, string, github.CreateCheckRunOptions) (*github.CheckRun, *github.Response, error)
	UpdateCheckRun(context.Context, string, string, int64, github.UpdateCheckRunOptions) (*github.CheckRun, *github.Response, error)
}

const (
	// https://github.community/t/undocumented-65535-character-limit-on-requests/117564
	defaultMaxFieldSize = 65535
	// Markdown markers for start and end of the text block containing terraform
	// output
	textStart = "```text\n"
	textEnd   = "\n```\n"
)

var (
	refreshingStateRegex = regexp.MustCompile("(?m)[\n\r]+^.*: Refreshing state... .*$")
)

// checkRunUpdate has all the info necessary to create or update a checkrun in
// the GH API.
type checkRunUpdate struct {
	*checkRun

	suite *v1alpha1.CheckSuite
	ws    *v1alpha1.Workspace
	run   *v1alpha1.Run

	// Logs are streamed into this byte array
	logs []byte

	reconcileErr error

	stripRefreshing bool

	// Max num of bytes github imposes on check run fields (summary, details)
	maxFieldSize int
}

// name() is the name given to the check run obj in the Github API.  Indicates
// the etok workspace for which the check run is running and summarises its
// current state. Github only shows the first 33 chars (and then an ellipsis) on
// the check runs page, so it's important to use those chars effectively.
func (u *checkRunUpdate) name() string {
	// Check run name always begins with full workspace name
	name := u.Namespace + "/" + u.ws.Name + " | "

	if u.id() == nil {
		// Until we have an ID the github client might create multiple check
		// runs, so it is important the name remains constant otherwise multiple
		// check runs will show up on the UI.
		return name + "planning"
	}

	switch u.command() {
	case planCmd:
		switch u.status() {
		case "completed":
			// Upon completion of a plan, instead of showing 'planned', show
			// summary of changes
			plan, err := parsePlanOutput(string(u.logs))
			if err != nil {
				// Just fallback to showing 'plan' and log error
				klog.Errorf("error parsing plan output for %s: %s", u.run, err.Error())
				name += "plan failed"
			} else {
				name += plan.summary()
			}
		default:
			name += "planning"
		}
	case applyCmd:
		switch u.status() {
		case "completed":
			name += "applied"
		default:
			name += "applying"
		}
	}

	return name
}

// Implements the invokable interface. Creates or updates a check run via the GH
// API depending upon whether a 'created' event has already been received.
func (u *checkRunUpdate) invoke(client checkRunClient) error {
	if u.id() != nil {
		if err := u.update(context.Background(), client); err != nil {
			return err
		}
		klog.InfoS("updated check run", "id", *u.id(), "status", u.status())
	} else {
		id, err := u.create(context.Background(), client)
		if err != nil {
			return err
		}
		klog.InfoS("created check run", "id", id, "status", u.status())
	}

	return nil
}

func (u *checkRunUpdate) create(ctx context.Context, client checkRunClient) (int64, error) {
	opts := github.CreateCheckRunOptions{
		Name:       u.name(),
		HeadSHA:    u.suite.Spec.SHA,
		Status:     github.String(u.status()),
		Conclusion: u.conclusion(),
		Output:     u.output(),
		Actions:    u.actions(),
		ExternalID: u.externalID(),
	}

	cr, _, err := client.CreateCheckRun(ctx, u.suite.Spec.Owner, u.suite.Spec.Repo, opts)
	return cr.GetID(), err
}

func (u *checkRunUpdate) update(ctx context.Context, client checkRunClient) error {
	opts := github.UpdateCheckRunOptions{
		Name:       u.name(),
		HeadSHA:    github.String(u.suite.Spec.SHA),
		Status:     github.String(u.status()),
		Conclusion: u.conclusion(),
		Output:     u.output(),
		Actions:    u.actions(),
		ExternalID: u.externalID(),
	}

	_, _, err := client.UpdateCheckRun(ctx, u.suite.Spec.Owner, u.suite.Spec.Repo, *u.id(), opts)
	return err
}

func (u *checkRunUpdate) actions() (actions []*github.CheckRunAction) {
	if u.status() != "completed" {
		return
	}

	actions = append(actions, &github.CheckRunAction{Label: "Plan", Description: "Re-run plan", Identifier: "plan"})

	if u.command() == planCmd {
		actions = append(actions, &github.CheckRunAction{Label: "Apply", Description: "Apply plan", Identifier: "apply"})
	}

	return
}

func (u *checkRunUpdate) externalID() *string {
	return github.String(u.Namespace + "/" + u.Name)
}

func (u *checkRunUpdate) output() *github.CheckRunOutput {
	return &github.CheckRunOutput{
		Title:   github.String(u.title()),
		Summary: github.String(u.summary()),
		Text:    u.details(),
	}
}

func (u *checkRunUpdate) status() string {
	if u.run == nil {
		return "queued"
	}
	if meta.IsStatusConditionTrue(u.run.Conditions, v1alpha1.RunFailedCondition) {
		return "completed"
	}
	if cond := meta.FindStatusCondition(u.run.Conditions, v1alpha1.RunCompleteCondition); cond != nil {
		if cond.Status == metav1.ConditionTrue {
			return "completed"
		}
		if cond.Status == metav1.ConditionFalse {
			if cond.Reason == v1alpha1.PodPendingReason || cond.Reason == v1alpha1.PodRunningReason {
				return "in_progress"
			}
		}
	}
	return "queued"
}

func (u *checkRunUpdate) conclusion() *string {
	if u.status() != "completed" {
		return nil
	}

	cond := u.run.Conditions[0]
	if cond.Type == v1alpha1.RunFailedCondition && cond.Status == metav1.ConditionTrue {
		if cond.Reason == v1alpha1.RunEnqueueTimeoutReason || cond.Reason == v1alpha1.QueueTimeoutReason {
			return github.String("timed_out")
		} else {
			return github.String("failure")
		}
	} else if cond.Type == v1alpha1.RunCompleteCondition && cond.Status == metav1.ConditionTrue {
		if cond.Reason == v1alpha1.PodFailedReason {
			return github.String("failure")
		} else {
			return github.String("success")
		}
	}
	return nil
}

func (u *checkRunUpdate) failureMessage() *string {
	if u.status() != "completed" {
		return nil
	}

	if len(u.run.Conditions) == 0 {
		return nil
	}

	cond := u.run.Conditions[0]
	if cond.Type == v1alpha1.RunFailedCondition && cond.Status == metav1.ConditionTrue {
		return &cond.Message
	}
	return nil
}

// Provide the 'title' of a check run
func (u *checkRunUpdate) title() string {
	return u.run.Name
}

// Populate the 'summary' text field of a check run
func (u *checkRunUpdate) summary() string {
	if msg := u.failureMessage(); msg != nil {
		return fmt.Sprintf("%s failed: %s\n", u.etokRunName(), *msg)
	}

	if u.reconcileErr != nil {
		return fmt.Sprintf("%s reconcile error: %s\n", u.Name, u.reconcileErr.Error())
	}

	return fmt.Sprintf("Note: you can also view logs by running: \n```bash\nkubectl logs -n %s pods/%s\n```", u.Namespace, u.etokRunName())
}

// Populate the 'details' text field of a check run
func (u *checkRunUpdate) details() *string {
	if u.reconcileErr != nil || u.failureMessage() != nil {
		// Terraform didn't even run so don't provide details
		return nil
	}

	if len(u.logs) == 0 {
		return nil
	}

	out := u.logs

	if u.stripRefreshing {
		// Replace 'refreshing...' lines
		out = refreshingStateRegex.ReplaceAll(out, []byte(""))
	}

	if (len(textStart) + len(out) + len(textEnd)) <= u.maxFieldSize {
		return github.String(textStart + string(bytes.TrimSpace(out)) + textEnd)
	}

	// Max bytes exceeded. Fetch new start position max bytes from end of
	// output.
	start := len(out) - u.maxFieldSize

	// Account for diff headers
	start += len(textStart)
	start += len(textEnd)

	// Add message explaining reason. The number of bytes skipped is inaccurate:
	// it doesn't account for additional bytes skipped in order to accommodate
	// this message.
	exceeded := fmt.Sprintf("--- exceeded limit of %d bytes; skipping first %d bytes ---\n", u.maxFieldSize, start)

	// Adjust start position to account for message
	start += len(exceeded)

	// Ensure output does not start half way through a line. Remove bytes
	// leading up to and including the first new line character.
	if i := bytes.IndexByte(out[start:], '\n'); i > -1 {
		start += i + 1
	}

	// Trim off any remaining leading or trailing new lines
	trimmed := bytes.Trim(out[start:], "\n")

	return github.String(textStart + exceeded + string(trimmed) + textEnd)
}
