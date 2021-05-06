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

// Represents a github check-run
type check struct {
	// Check ID is only populated after the gh client creates the check.  Once
	// populated, we can use it to update the check.
	id *int64

	// Etok run name and namespace. Run name is only populated once a check is
	// created.
	run, namespace string

	// Previous etok run name - populated if check is re-run or applied
	previous string

	stripRefreshing bool

	// Error creating k8s resources
	createErr error

	// Run failure message - its pod didn't even start
	runFailure *string

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
}

// Construct check from run resource.
func newCheckFromResource(res *v1alpha1.Run, createErr error) (*check, error) {
	lbls := res.GetLabels()
	if lbls == nil {
		return nil, fmt.Errorf("no labels found on run resource")
	}
	idStr, ok := lbls[checkIDLabelName]
	if !ok {
		return nil, fmt.Errorf("run %s missing label %s", klog.KObj(res), checkIDLabelName)
	}
	sha, ok := lbls[checkSHALabelName]
	if !ok {
		return nil, fmt.Errorf("run %s missing label %s", klog.KObj(res), checkSHALabelName)
	}
	owner, ok := lbls[checkOwnerLabelName]
	if !ok {
		return nil, fmt.Errorf("run %s missing label %s", klog.KObj(res), checkOwnerLabelName)
	}
	repo, ok := lbls[checkRepoLabelName]
	if !ok {
		return nil, fmt.Errorf("run %s missing label %s", klog.KObj(res), checkRepoLabelName)
	}
	cmd, ok := lbls[checkCommandLabelName]
	if !ok {
		return nil, fmt.Errorf("run %s missing label %s", klog.KObj(res), checkCommandLabelName)
	}

	id, err := strconv.ParseInt(idStr, 10, 64)
	if !ok {
		return nil, fmt.Errorf("unable to parse label value: %s: %s=%s: %w", klog.KObj(res), checkRepoLabelName, idStr, err)
	}

	r := check{
		id:           &id,
		run:          res.Name,
		namespace:    res.Namespace,
		sha:          sha,
		owner:        owner,
		repo:         repo,
		command:      cmd,
		workspace:    res.Workspace,
		maxFieldSize: defaultMaxFieldSize,
		createErr:    createErr,
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

		r.out = []byte(fmt.Sprintf("Run failed: %s\n", cond.Message))

		r.runFailure = github.String(cond.Message)

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
func (r *check) invoke(client *GithubClient) error {
	op := checkOperation{
		check: r,
	}

	if r.status != nil && *r.status == "completed" {
		op.setAction("Plan", "Re-run plan", "plan")

		if r.command == "plan" {
			op.setAction("Apply", "Apply plan", "apply")
		}
	}

	if r.id != nil {
		err := op.update(context.Background(), client, *r.id)
		if err != nil {
			klog.Errorf("unable to update check run: %s", err.Error())
			return err
		}
		klog.InfoS("updated check run", "id", *r.id, "ref", r.sha)
	} else {
		id, err := op.create(context.Background(), client)
		if err != nil {
			klog.Errorf("unable to create check run: %s", err.Error())
			return err
		}
		r.id = &id
		klog.InfoS("created check run", "id", id, "ref", r.sha)
	}

	return nil
}

// Provide name for check run. Indicates the etok workspace for which the check
// run is running and summarises its current state. Github only shows the first
// 33 chars (and then an ellipsis) on the check runs page, so it's important to
// use those chars effectively.
func (r *check) name() string {
	// Check run name always begins with full workspace name
	name := r.namespace + "/" + r.workspace + " | "

	switch {
	case r.createErr != nil:
		name += r.command + " failed"
	case r.command == "plan":
		if r.status == nil {
			name += "planning"
		} else if *r.status == "completed" {
			// Upon completion of a plan, instead of showing 'planned', show
			// summary of changes
			plan, err := parsePlanOutput(string(r.out))
			if err != nil {
				// Just fallback to showing 'plan' and log error
				klog.Errorf("error parsing plan output for %s: %s", r.run, err.Error())
				name += "plan failed"
			} else {
				name += plan.summary()
			}
		} else {
			name += "planning"
		}
	case r.command == "apply":
		if r.status == nil {
			name += "applying"
		} else if *r.status == "completed" {
			name += "applied"
		} else {
			name += "applying"
		}
	}

	return name
}

// Provide the 'title' of a check run
func (r *check) title() string {
	return r.run
}

// Populate the 'summary' text field of a check run
func (r *check) summary() string {
	if r.createErr != nil {
		return fmt.Sprintf("Unable to create kubernetes resources: %s\n", r.createErr.Error())
	}

	if r.runFailure != nil {
		return fmt.Sprintf("%s failed: %s\n", r.run, *r.runFailure)
	}

	return fmt.Sprintf("Note: you can also view logs by running: \n```bash\nkubectl logs -n %s pods/%s\n```", r.namespace, r.run)
}

// Populate the 'details' text field of a check run
func (r *check) details() *string {
	if r.createErr != nil || r.runFailure != nil {
		// Terraform didn't even run so don't provide details
		return nil
	}

	out := r.out

	if r.stripRefreshing {
		// Replace 'refreshing...' lines
		out = refreshingStateRegex.ReplaceAll(out, []byte(""))
	}

	if (len(textStart) + len(out) + len(textEnd)) <= r.maxFieldSize {
		return github.String(textStart + string(bytes.TrimSpace(out)) + textEnd)
	}

	// Max bytes exceeded. Fetch new start position max bytes from end of
	// output.
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

	return github.String(textStart + exceeded + string(trimmed) + textEnd)
}
