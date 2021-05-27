package github

import (
	"errors"
	"testing"

	"github.com/google/go-github/v31/github"
	"github.com/leg100/etok/api/etok.dev/v1alpha1"
	"github.com/leg100/etok/pkg/builders"
	"github.com/leg100/etok/pkg/testobj"
	"github.com/leg100/etok/pkg/testutil"
	"github.com/stretchr/testify/assert"
)

func TestCheckRunUpdate(t *testing.T) {
	u := checkRunUpdate{
		checkRun:     &checkRun{builders.CheckRun().Suite(12345, 0).Namespace("dev").Workspace("networks").Build()},
		suite:        builders.CheckSuite(12345).Build(),
		logs:         make([]byte, 0),
		maxFieldSize: defaultMaxFieldSize,
		run:          &v1alpha1.Run{},
		ws:           testobj.Workspace("dev", "networks"),
	}

	testutil.Run(t, "no logs", func(t *testutil.T) {
		assert.Nil(t, u.details())
		assert.Equal(t,
			"Note: you can also view logs by running: \n```bash\nkubectl logs -n dev pods/12345-0-networks-0\n```",
			u.summary())
	})

	testutil.Run(t, "some logs", func(t *testutil.T) {
		t.Override(&u.logs, []byte(`line 1
line 2
line 3
`))
		assert.Equal(t,
			"```text\nline 1\nline 2\nline 3\n```\n",
			*u.details())
	})

	testutil.Run(t, "strip refreshing lines", func(t *testutil.T) {
		t.Override(&u.stripRefreshing, true)
		t.Override(&u.logs, t.ReadFile("fixtures/got.txt"))

		assert.Equal(t, t.ReadFile("fixtures/want_without_refresh.md"), []byte(*u.details()))
	})

	testutil.Run(t, "exceed max field size", func(t *testutil.T) {
		t.Override(&u.maxFieldSize, 1000)
		t.Override(&u.logs, t.ReadFile("fixtures/got.txt"))

		assert.Equal(t, string(t.ReadFile("fixtures/want_truncated.md")), *u.details())
	})

	testutil.Run(t, "reconciler error", func(t *testutil.T) {
		t.Override(&u.reconcileErr, errors.New("unable to create run resource"))

		assert.Equal(t, "12345-0-networks reconcile error: unable to create run resource\n", u.summary())
		assert.Nil(t, u.details())
	})

	testutil.Run(t, "failed run", func(t *testutil.T) {
		t.Override(&u.run,
			testobj.Run("dev", "12345-networks-0", "plan",
				testobj.WithCondition(v1alpha1.RunFailedCondition, v1alpha1.RunEnqueueTimeoutReason, "run failed to enqueue in time")))

		assert.Equal(t, "completed", u.status())
		assert.Equal(t, "timed_out", *u.conclusion())
		assert.Equal(t, "12345-0-networks-0 failed: run failed to enqueue in time\n", u.summary())
		assert.Nil(t, u.details())
	})

	testutil.Run(t, "initial name", func(t *testutil.T) {
		assert.Equal(t, "dev/networks | planning", u.name())
	})

	testutil.Run(t, "successfully completed plan", func(t *testutil.T) {
		t.Override(&u.run,
			testobj.Run("dev", "12345-0-networks-0", "plan", testobj.WithCondition(v1alpha1.RunCompleteCondition)))
		t.Override(&u.logs, t.ReadFile("fixtures/plan.txt"))
		t.Override(&u.Status.Events, []*v1alpha1.CheckRunEvent{
			{
				Created: &v1alpha1.CheckRunCreatedEvent{ID: 987},
			},
		})

		assert.Equal(t, "completed", u.status())
		assert.Equal(t, "success", *u.conclusion())
		assert.Equal(t, "dev/networks | +2/~0/−0", u.name())
		assert.Equal(t, []*github.CheckRunAction{
			{Label: "Plan", Description: "Re-run plan", Identifier: "plan"},
		}, u.actions())

		// Apply button should be visible once pull is mergeable
		t.Override(&u.suite.Status.Mergeable, true)
		assert.Contains(t, u.actions(), &github.CheckRunAction{
			Label: "Apply", Description: "Apply plan", Identifier: "apply",
		})
	})

	testutil.Run(t, "incomplete apply", func(t *testutil.T) {
		// Run w/o Completed Condition
		t.Override(&u.run, testobj.Run("dev", "12345-0-networks-0", "apply"))

		t.Override(&u.logs, t.ReadFile("fixtures/plan.txt"))
		t.Override(&u.Status.Events, []*v1alpha1.CheckRunEvent{
			{
				Created: &v1alpha1.CheckRunCreatedEvent{ID: 987},
			},
			{
				RequestedAction: &v1alpha1.CheckRunRequestedActionEvent{Action: "apply"},
			},
		})

		assert.Equal(t, "dev/networks | applying", u.name())
		assert.Empty(t, u.actions())
	})

	testutil.Run(t, "successfully completed apply", func(t *testutil.T) {
		t.Override(&u.run,
			testobj.Run("dev", "12345-0-networks-0", "apply", testobj.WithCondition(v1alpha1.RunCompleteCondition)))
		t.Override(&u.logs, t.ReadFile("fixtures/plan.txt"))
		t.Override(&u.Status.Events, []*v1alpha1.CheckRunEvent{
			{
				Created: &v1alpha1.CheckRunCreatedEvent{ID: 987},
			},
			{
				RequestedAction: &v1alpha1.CheckRunRequestedActionEvent{Action: "apply"},
			},
		})

		assert.Equal(t, "completed", u.status())
		assert.Equal(t, "success", *u.conclusion())
		assert.Equal(t, "dev/networks | applied", u.name())
		assert.Equal(t, []*github.CheckRunAction{
			{Label: "Plan", Description: "Re-run plan", Identifier: "plan"},
		}, u.actions())
	})
}
