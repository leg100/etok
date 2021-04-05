package github

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/leg100/etok/api/etok.dev/v1alpha1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/klog/v2"
)

const (
	// https://github.community/t/undocumented-65535-character-limit-on-requests/117564
	defaultMaxFieldSize = 65535
)

// Represents a github checkrun
type checkRun struct {
	id, namespace string

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

	// Github checkrun status
	status string

	// Github checkrun conclusion
	conclusion string

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

	r := checkRun{
		id:        res.Name,
		namespace: res.Namespace,
		sha:       sha,
		owner:     owner,
		repo:      repo,
		workspace: res.Workspace,
		err:       createErr,
	}

	if createErr != nil {
		// Failed to create k8s resources
		r.status = "completed"
		r.conclusion = "failure"
	}

	if meta.IsStatusConditionTrue(res.Conditions, v1alpha1.RunFailedCondition) {
		r.status = "completed"

		cond := meta.FindStatusCondition(res.Conditions, v1alpha1.RunFailedCondition)
		switch cond.Reason {
		case v1alpha1.RunEnqueueTimeoutReason, v1alpha1.QueueTimeoutReason:
			r.conclusion = "timed_out"
		default:
			r.conclusion = "failed"
		}

	} else if meta.IsStatusConditionTrue(res.Conditions, v1alpha1.RunCompleteCondition) {
		r.status = "completed"

		cond := meta.FindStatusCondition(res.Conditions, v1alpha1.RunCompleteCondition)
		switch cond.Reason {
		case v1alpha1.PodFailedReason:
			r.conclusion = "failed"
		default:
			r.conclusion = "success"
		}

	} else if meta.IsStatusConditionFalse(res.Conditions, v1alpha1.RunCompleteCondition) {

		cond := meta.FindStatusCondition(res.Conditions, v1alpha1.RunCompleteCondition)
		switch cond.Reason {
		case v1alpha1.RunQueuedReason, v1alpha1.RunUnqueuedReason:
			r.status = "queued"
		case v1alpha1.PodRunningReason, v1alpha1.PodPendingReason:
			r.status = "in_progress"
		}
	} else {
		r.status = "queued"
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

	if r.status == "completed" {
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

	// Next part of name is the command name
	command := r.command
	if r.command == "plan" && r.status == "completed" {
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
	name += fmt.Sprintf("[%s]", command)

	// Next part is the run id (run-123yx). GH is likely to cut this short with
	// a '...' so snip off the redundant prefix 'run-' and just show the ID.
	// That way the ID - the important bit - is more likely to be visible to the
	// user.
	var id string
	idparts := strings.Split(r.id, "-")
	if len(idparts) == 2 {
		id = idparts[1]
	}
	name += " " + id

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
	diffStart := "```diff\n"
	diffEnd := "\n```\n"

	if (len(diffStart) + len(r.out) + len(diffEnd)) <= r.maxFieldSize {
		return diffStart + string(bytes.TrimSpace(r.out)) + diffEnd
	}

	// Max bytes exceeded. Fetch new start position max bytes into output.
	start := len(r.out) - r.maxFieldSize

	// Account for diff headers
	start += len(diffStart)
	start += len(diffEnd)

	// Add message explaining reason. The number of bytes skipped is inaccurate:
	// it doesn't account for additional bytes skipped in order to accommodate
	// this message.
	exceeded := fmt.Sprintf("--- exceeded limit of %d bytes; skipping first %d bytes ---\n", r.maxFieldSize, start)

	// Adjust start position to account for message
	start += len(exceeded)

	// Ensure output does not start half way through a line. Remove bytes
	// leading up to and including the first new line character.
	if i := bytes.IndexByte(r.out[start:], '\n'); i > -1 {
		start += i + 1
	}

	// Trim off any remaining leading or trailing new lines
	trimmed := bytes.Trim(r.out[start:], "\n")

	return diffStart + exceeded + string(trimmed) + diffEnd
}

// Write implements io.Writer. The launcher calls Write with the logs it streams
// from the pod. As well as storing the logs to an internal buffer for
// populating the text fields of the check run, it provides an opportunity to
// strip out unnecessary content.
func (cr *checkRun) Write(p []byte) (int, error) {
	// Total bytes written
	var written int

	r := bufio.NewReader(bytes.NewBuffer(p))
	// Read segments of bytes delimited with a new line.
	for {
		line, err := r.ReadBytes('\n')
		written += len(line)
		if err == io.EOF {
			return written, nil
		}
		if err != nil {
			return written, err
		}

		if cr.stripRefreshing && bytes.Contains(line, []byte(": Refreshing state... ")) {
			continue
		}

		if bytes.HasPrefix(line, []byte("  +")) || bytes.HasPrefix(line, []byte("  -")) || bytes.HasPrefix(line, []byte("  ~")) {
			// Trigger diff color highlighting by unindenting lines beginning
			// with +/-/~
			line = bytes.TrimLeft(line, " ")
		}

		cr.out = append(cr.out, line...)
	}
}
