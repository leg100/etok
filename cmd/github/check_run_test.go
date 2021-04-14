package github

import (
	"io/ioutil"
	"testing"

	"github.com/google/go-github/v31/github"
	"github.com/leg100/etok/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunOutput(t *testing.T) {
	tests := []struct {
		name      string
		id        string
		command   string
		namespace string
		workspace string
		run       checkRun
		iteration int
		// Path to fixture from which to populate output buffer
		out      string
		wantFile string
	}{
		{
			name: "within maximum size",
			out:  "fixtures/got.txt",
			run: checkRun{
				maxFieldSize: defaultMaxFieldSize,
			},
			wantFile: "fixtures/want.md",
		},
		{
			name: "exceeds maximum size",
			out:  "fixtures/got.txt",
			run: checkRun{
				// Default is 64k but we'll set to an artificially low number so
				// that we can easily test this maximum being breached
				maxFieldSize: 1000,
			},
			wantFile: "fixtures/want_truncated.md",
		},
		{
			name: "strip off refreshing lines",
			out:  "fixtures/got.txt",
			run: checkRun{
				maxFieldSize:    defaultMaxFieldSize,
				stripRefreshing: true,
			},
			wantFile: "fixtures/want_without_refresh.md",
		},
	}
	for _, tt := range tests {
		testutil.Run(t, tt.name, func(t *testutil.T) {
			// Write output to checkrun's output buffer
			out, err := ioutil.ReadFile(tt.out)
			require.NoError(t, err)
			tt.run.out = out

			want, err := ioutil.ReadFile(tt.wantFile)
			require.NoError(t, err)

			assert.Equal(t, string(want), tt.run.details())
		})
	}
}

func TestRunName(t *testing.T) {
	tests := []struct {
		name      string
		id        string
		command   string
		namespace string
		workspace string
		status    *string
		iteration int
		want      string
		// Path to fixture from which to populate output buffer
		out string
	}{
		{
			name:      "incomplete plan",
			id:        "run-12345",
			command:   "plan",
			namespace: "default",
			workspace: "default",
			want:      "default/default #0 [plan]",
		},
		{
			name:      "completed plan",
			id:        "run-12345",
			command:   "plan",
			status:    github.String("completed"),
			namespace: "default",
			workspace: "default",
			out:       "fixtures/plan.txt",
			want:      "default/default #0 [+2~0-0]",
		},
		{
			name:      "second iteration",
			id:        "run-12345",
			command:   "plan",
			status:    github.String("completed"),
			namespace: "default",
			workspace: "default",
			iteration: 1,
			out:       "fixtures/plan.txt",
			want:      "default/default #1 [+2~0-0]",
		},
		{
			name:      "completed plan, no changes",
			id:        "run-12345",
			command:   "plan",
			status:    github.String("completed"),
			namespace: "default",
			workspace: "default",
			out:       "fixtures/plan_no_changes.txt",
			want:      "default/default #0 [+0~0-0]",
		},
		{
			name:      "apply",
			id:        "run-12345",
			command:   "apply",
			namespace: "default",
			workspace: "default",
			want:      "default/default #0 [apply]",
		},
	}

	for _, tt := range tests {
		testutil.Run(t, tt.name, func(t *testutil.T) {
			r := checkRun{
				command:   tt.command,
				namespace: tt.namespace,
				workspace: tt.workspace,
				status:    tt.status,
				iteration: tt.iteration,
				id:        tt.id,
			}

			if tt.out != "" {
				r.out, _ = ioutil.ReadFile(tt.out)
			}

			assert.Equal(t, tt.want, r.name())
		})
	}
}
