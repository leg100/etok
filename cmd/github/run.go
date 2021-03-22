package github

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/google/go-github/v31/github"
	"github.com/leg100/etok/api/etok.dev/v1alpha1"
	"github.com/leg100/etok/pkg/launcher"
	"k8s.io/klog/v2"
)

const (
	// https://github.community/t/undocumented-65535-character-limit-on-requests/117564
	defaultMaxFieldSize = 65535
)

type run struct {
	id, namespace   string
	stripRefreshing bool
	completed       bool
	err             error

	// The sha of the git commit that triggered this run
	sha string

	// Logs are streamed into this byte array
	out []byte

	// The github checkrun command
	command string

	maxFieldSize int

	// Check Run ID is only populated after the gh client creates the check run.
	// Once populated, we can use it to update the check run.
	checkRunId *int64
}

func newRun(opts *launcher.LauncherOptions, sha string) (*run, error) {
	return &run{
		id:        opts.RunName,
		namespace: opts.Namespace,
		sha:       sha,
	}, nil
}

func newRunFromResource(res *v1alpha1.Run) (*run, error) {
	lbls := res.GetLabels()
	if lbls == nil {
		return nil, fmt.Errorf("no labels found on run resource")
	}
	sha, ok := lbls[checkRunSHALabelName]
	if !ok {
		return nil, fmt.Errorf("run %s missing label %s", klog.KObj(res), checkRunSHALabelName)
	}

	return &run{
		id:        res.Name,
		namespace: res.Namespace,
		sha:       sha,
	}, nil
}

// Update actually updates the check run on GH. It does so idempotently: if the
// CR is yet to be created it will be created, and if it's already created,
// it'll be updated.
func (r *run) invoke(client *GithubClient) error {
	op := etokRunOperation{
		etokRun: r,
	}

	if r.completed {
		op.setAction("Plan", "Re-run plan", "plan")

		if r.command == "plan" {
			op.setAction("Apply", "Apply plan", "apply")
		}

		if r.err != nil {
			op.conclusion = github.String("failure")
		} else {
			op.conclusion = github.String("success")
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
func (r *run) name() string {
	// Check run name always begins with full workspace name
	name := r.namespace + "/" + r.id

	// Next part of name is the command name
	command := r.command
	if r.command == "plan" && r.completed {
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

// Provide current status of check run
func (u *run) status() *string {
	if u.completed {
		return github.String("completed")
	}
	return github.String("in_progress")
}

// Provide the 'title' of a check run
func (u *run) title() string {
	return u.id
}

// Provide the external ID of a check run
func (u *run) externalID() *string {
	return github.String(u.namespace + "/" + u.id)
}

// Populate the 'summary' text field of a check run
func (u *run) summary() string {
	if u.err != nil {
		return u.err.Error()
	}

	return fmt.Sprintf("Run `kubectl logs -f pods/%s`", u.id)
}

// Populate the 'details' text field of a check run
func (o *run) details() string {
	diffStart := "```diff\n"
	diffEnd := "\n```\n"

	if (len(diffStart) + len(o.out) + len(diffEnd)) <= o.maxFieldSize {
		return diffStart + string(bytes.TrimSpace(o.out)) + diffEnd
	}

	// Max bytes exceeded. Fetch new start position max bytes into output.
	start := len(o.out) - o.maxFieldSize

	// Account for diff headers
	start += len(diffStart)
	start += len(diffEnd)

	// Add message explaining reason. The number of bytes skipped is inaccurate:
	// it doesn't account for additional bytes skipped in order to accommodate
	// this message.
	exceeded := fmt.Sprintf("--- exceeded limit of %d bytes; skipping first %d bytes ---\n", o.maxFieldSize, start)

	// Adjust start position to account for message
	start += len(exceeded)

	// Ensure output does not start half way through a line. Remove bytes
	// leading up to and including the first new line character.
	if i := bytes.IndexByte(o.out[start:], '\n'); i > -1 {
		start += i + 1
	}

	// Trim off any remaining leading or trailing new lines
	trimmed := bytes.Trim(o.out[start:], "\n")

	return diffStart + exceeded + string(trimmed) + diffEnd
}

// Write implements io.Writer. The launcher calls Write with the logs it streams
// from the pod. As well as storing the logs to an internal buffer for
// populating the text fields of the check run, it provides an opportunity to
// strip out unnecessary content.
func (o *run) Write(p []byte) (int, error) {
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

		if o.stripRefreshing && bytes.Contains(line, []byte(": Refreshing state... ")) {
			continue
		}

		if bytes.HasPrefix(line, []byte("  +")) || bytes.HasPrefix(line, []byte("  -")) || bytes.HasPrefix(line, []byte("  ~")) {
			// Trigger diff color highlighting by unindenting lines beginning
			// with +/-/~
			line = bytes.TrimLeft(line, " ")
		}

		o.out = append(o.out, line...)
	}
}
