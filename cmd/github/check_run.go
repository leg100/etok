package github

import (
	"bytes"
	"context"
	"fmt"
	"regexp"
	"strconv"

	"github.com/google/go-github/v31/github"
	"github.com/leg100/etok/api/etok.dev/v1alpha1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/klog/v2"
)

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

// Represents a github checkrun
type checkRun struct {
	id, namespace string

	// Iteration is the ordinal number of times a plan/apply has been run, for
	// this workspace, for this checksuite. Its only purpose is to ensure check
	// runs are presented in a logical order on the github checks UI, which
	// orders them alphanumerically, with the iteration being appended to the
	// check run name.
	iteration int

	// Previous etok run ID - populated if check run is a re-run or apply of a
	// previous run
	previous string

	stripRefreshing bool
	err             error

	// The workspace of the run
	workspace string

	// The sha of the git commit that triggered this run
	sha string

	// The owner of the repo for the checkrun
	owner string

	// The repo name for the checkrun
	repo string

	// Logs are streamed into this byte array
	out []byte

	// The github checkrun command
	command string

	// Github checkrun status - nil equates to 'queued'
	status *string

	// Github checkrun conclusion
	conclusion *string

	// Max num of bytes github imposes on check run fields (summary, details)
	maxFieldSize int

	// Check Run ID is only populated after the gh client creates the check run.
	// Once populated, we can use it to update the check run.
	checkRunId *int64
}

// Construct check run from run resource.
func newRunFromResource(res *v1alpha1.Run, createErr error) (*checkRun, error) {
	lbls := res.GetLabels()
	if lbls == nil {
		return nil, fmt.Errorf("no labels found on run resource")
	}
	idStr, ok := lbls[checkRunIDLabelName]
	if !ok {
		return nil, fmt.Errorf("run %s missing label %s", klog.KObj(res), checkRunIDLabelName)
	}
	sha, ok := lbls[checkRunSHALabelName]
	if !ok {
		return nil, fmt.Errorf("run %s missing label %s", klog.KObj(res), checkRunSHALabelName)
	}
	owner, ok := lbls[checkRunOwnerLabelName]
	if !ok {
		return nil, fmt.Errorf("run %s missing label %s", klog.KObj(res), checkRunOwnerLabelName)
	}
	repo, ok := lbls[checkRunRepoLabelName]
	if !ok {
		return nil, fmt.Errorf("run %s missing label %s", klog.KObj(res), checkRunRepoLabelName)
	}
	cmd, ok := lbls[checkRunCommandLabelName]
	if !ok {
		return nil, fmt.Errorf("run %s missing label %s", klog.KObj(res), checkRunCommandLabelName)
	}
	iterationStr, ok := lbls[checkRunIterationLabelName]
	if !ok {
		return nil, fmt.Errorf("run %s missing label %s", klog.KObj(res), checkRunIterationLabelName)
	}

	id, err := strconv.ParseInt(idStr, 10, 64)
	if !ok {
		return nil, fmt.Errorf("unable to parse label value: %s: %s=%s: %w", klog.KObj(res), checkRunRepoLabelName, idStr, err)
	}

	iteration, err := strconv.ParseInt(iterationStr, 10, 0)
	if !ok {
		return nil, fmt.Errorf("unable to parse label value: %s: %s=%s: %w", klog.KObj(res), checkRunIterationLabelName, iterationStr, err)
	}

	r := checkRun{
		id:           res.Name,
		checkRunId:   &id,
		namespace:    res.Namespace,
		sha:          sha,
		owner:        owner,
		repo:         repo,
		command:      cmd,
		workspace:    res.Workspace,
		maxFieldSize: defaultMaxFieldSize,
		iteration:    int(iteration),
		err:          createErr,
	}

	if createErr != nil {
		// Failed to create k8s resources
		r.status = github.String("completed")
		r.conclusion = github.String("failure")
		return &r, nil
	}

	if meta.IsStatusConditionTrue(res.Conditions, v1alpha1.RunFailedCondition) {
		r.status = github.String("completed")

		cond := meta.FindStatusCondition(res.Conditions, v1alpha1.RunFailedCondition)
		switch cond.Reason {
		case v1alpha1.RunEnqueueTimeoutReason, v1alpha1.QueueTimeoutReason:
			r.conclusion = github.String("timed_out")
		default:
			r.conclusion = github.String("failure")
		}

	} else if meta.IsStatusConditionTrue(res.Conditions, v1alpha1.RunCompleteCondition) {
		r.status = github.String("completed")

		cond := meta.FindStatusCondition(res.Conditions, v1alpha1.RunCompleteCondition)
		switch cond.Reason {
		case v1alpha1.PodFailedReason:
			r.conclusion = github.String("failure")
		default:
			r.conclusion = github.String("success")
		}

	} else if meta.IsStatusConditionFalse(res.Conditions, v1alpha1.RunCompleteCondition) {

		cond := meta.FindStatusCondition(res.Conditions, v1alpha1.RunCompleteCondition)
		switch cond.Reason {
		case v1alpha1.RunQueuedReason, v1alpha1.RunUnqueuedReason:
			r.status = github.String("queued")
		case v1alpha1.PodRunningReason, v1alpha1.PodPendingReason:
			r.status = github.String("in_progress")
		}
	}

	return &r, nil
}

// Update actually updates the check run on GH. It does so idempotently: if the
// CR is yet to be created it will be created, and if it's already created,
// it'll be updated.
func (r *checkRun) invoke(client *GithubClient) error {
	op := checkRunOperation{
		checkRun: r,
	}

	if r.status != nil && *r.status == "completed" {
		op.setAction("Plan", "Re-run plan", "plan")

		if r.command == "plan" {
			op.setAction("Apply", "Apply plan", "apply")
		}
	}

	if r.checkRunId != nil {
		err := op.update(context.Background(), client, *r.checkRunId)
		if err != nil {
			klog.Errorf("unable to update check run: %s", err.Error())
			return err
		}
		klog.InfoS("updated check run", "id", *r.checkRunId, "ref", r.sha)
	} else {
		id, err := op.create(context.Background(), client)
		if err != nil {
			klog.Errorf("unable to create check run: %s", err.Error())
			return err
		}
		r.checkRunId = &id
		klog.InfoS("created check run", "id", id, "ref", r.sha)
	}

	return nil
}

// Provide name for check run. Indicates the etok workspace for which the check
// run is running and summarises its current state. Github only shows the first
// 33 chars (and then an ellipsis) on the check runs page, so it's important to
// use those chars effectively.
func (r *checkRun) name() string {
	// Check run name always begins with full workspace name
	name := r.namespace + "/" + r.workspace

	// And we append iteration to ensure check run is unique and ordered
	name += fmt.Sprintf(" #%d ", r.iteration)

	// Next part of name is the command name
	command := r.command
	if r.command == "plan" && r.err == nil && r.status != nil && *r.status == "completed" {
		// Upon completion of a plan, instead of showing 'plan', show summary of
		// changes
		plan, err := parsePlanOutput(string(r.out))
		if err != nil {
			// Just fallback to showing 'plan' and log error
			klog.Errorf("error parsing plan output for %s: %s", r.id, err.Error())
		} else {
			command = plan.summary()
		}
	}
	name += command

	return name
}

// Provide the 'title' of a check run
func (r *checkRun) title() string {
	return r.id
}

// Provide the external ID of a check run
func (r *checkRun) externalID() *string {
	metadata := CheckRunMetadata{
		Command:   r.command,
		Current:   r.id,
		Namespace: r.namespace,
		Previous:  r.previous,
		Workspace: r.workspace,
		Iteration: r.iteration,
	}
	return metadata.ToStringPtr()
}

// Populate the 'summary' text field of a check run
func (r *checkRun) summary() string {
	if r.err != nil {
		return r.err.Error()
	}

	return fmt.Sprintf("Run `kubectl logs -f pods/%s`", r.id)
}

// Populate the 'details' text field of a check run
func (r *checkRun) details() string {
	out := r.out

	if r.stripRefreshing {
		// Replace 'refreshing...' lines
		out = refreshingStateRegex.ReplaceAll(r.out, []byte(""))
	}

	if (len(textStart) + len(out) + len(textEnd)) <= r.maxFieldSize {
		return textStart + string(bytes.TrimSpace(out)) + textEnd
	}

	// Max bytes exceeded. Fetch new start position max bytes into output.
	start := len(out) - r.maxFieldSize

	// Account for diff headers
	start += len(textStart)
	start += len(textEnd)

	// Add message explaining reason. The number of bytes skipped is inaccurate:
	// it doesn't account for additional bytes skipped in order to accommodate
	// this message.
	exceeded := fmt.Sprintf("--- exceeded limit of %d bytes; skipping first %d bytes ---\n", r.maxFieldSize, start)

	// Adjust start position to account for message
	start += len(exceeded)

	// Ensure output does not start half way through a line. Remove bytes
	// leading up to and including the first new line character.
	if i := bytes.IndexByte(out[start:], '\n'); i > -1 {
		start += i + 1
	}

	// Trim off any remaining leading or trailing new lines
	trimmed := bytes.Trim(out[start:], "\n")

	return textStart + exceeded + string(trimmed) + textEnd
}

// Write implements io.Writer. The launcher calls Write with the logs it streams
// from the pod. As well as storing the logs to an internal buffer for
// populating the text fields of the check run, it provides an opportunity to
// strip out unnecessary content.
func (cr *checkRun) Write(p []byte) (int, error) {
	cr.out = p
	return len(p), nil
}
