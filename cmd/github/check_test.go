package github

import (
	"errors"
	"io/ioutil"
	"testing"

	"github.com/google/go-github/v31/github"
	"github.com/leg100/etok/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckRunSummary(t *testing.T) {
	tests := []struct {
		name string
		run  check
		want string
	}{
		{
			name: "default",
			run: check{
				run:       "run-12345",
				namespace: "dev",
			},
			want: "Note: you can also view logs by running: \n```bash\nkubectl logs -n dev pods/run-12345\n```",
		},
		{
			name: "create error",
			run: check{
				createErr: errors.New("unable to create resources"),
			},
			want: "Unable to create kubernetes resources: unable to create resources\n",
		},
		{
			name: "run failure",
			run: check{
				run:        "run-12345",
				runFailure: github.String("run timeout"),
			},
			want: "run-12345 failed: run timeout\n",
		},
	}
	for _, tt := range tests {
		testutil.Run(t, tt.name, func(t *testutil.T) {
			assert.Equal(t, tt.want, tt.run.summary())
		})
	}
}

func TestCheckRunDetails(t *testing.T) {
	tests := []struct {
		name      string
		id        string
		command   string
		namespace string
		workspace string
		run       check
		// Path to fixture from which to populate output buffer
		out string
		// Want contents of file
		wantFile string
		// Want string
		want string
		// Want nil
		wantNil bool
	}{
		{
			name: "within maximum size",
			out:  "fixtures/got.txt",
			run: check{
				maxFieldSize: defaultMaxFieldSize,
			},
			wantFile: "fixtures/want.md",
		},
		{
			name: "exceeds maximum size",
			out:  "fixtures/got.txt",
			run: check{
				// Default is 64k but we'll set it to an artificially low number
				// in order to breach the maximum
				maxFieldSize: 1000,
			},
			wantFile: "fixtures/want_truncated.md",
		},
		{
			name: "exceeds maximum size massively",
			out:  "fixtures/big-plan.txt",
			run: check{
				// Default is 64k but we'll set it to an artificially low number
				// in order to breach the maximum
				maxFieldSize: defaultMaxFieldSize,
			},
			wantFile: "fixtures/big-plan-truncated.txt",
		},
		{
			name: "strip off refreshing lines",
			out:  "fixtures/got.txt",
			run: check{
				maxFieldSize:    defaultMaxFieldSize,
				stripRefreshing: true,
			},
			wantFile: "fixtures/want_without_refresh.md",
		},
		{
			name: "do not provide details when unable to create resources",
			run: check{
				createErr: errors.New("unable to create resources"),
			},
			wantNil: true,
		},
		{
			name: "do not provide details when there is a run failure",
			run: check{
				runFailure: github.String("run timeout"),
			},
			wantNil: true,
		},
	}
	for _, tt := range tests {
		testutil.Run(t, tt.name, func(t *testutil.T) {
			if tt.out != "" {
				// Write output to checkrun's output buffer
				out, err := ioutil.ReadFile(tt.out)
				require.NoError(t, err)
				tt.run.out = out
			}

			if tt.want != "" {
				assert.Equal(t, tt.want, *tt.run.details())
			}

			if tt.wantFile != "" {
				want, err := ioutil.ReadFile(tt.wantFile)
				require.NoError(t, err)

				assert.Equal(t, string(want), *tt.run.details())
			}

			if tt.wantNil {
				assert.Nil(t, tt.run.details())
			}
		})
	}
}

func TestCheckRunName(t *testing.T) {
	tests := []struct {
		name      string
		id        string
		command   string
		namespace string
		workspace string
		status    *string
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
			want:      "default/default | planning",
		},
		{
			name:      "completed plan",
			id:        "run-12345",
			command:   "plan",
			status:    github.String("completed"),
			namespace: "default",
			workspace: "default",
			out:       "fixtures/plan.txt",
			want:      "default/default | +2/~0/−0",
		},
		{
			name:      "completed plan, no changes",
			id:        "run-12345",
			command:   "plan",
			status:    github.String("completed"),
			namespace: "default",
			workspace: "default",
			out:       "fixtures/plan_no_changes.txt",
			want:      "default/default | +0/~0/−0",
		},
		{
			name:      "apply",
			id:        "run-12345",
			command:   "apply",
			namespace: "default",
			workspace: "default",
			want:      "default/default | applying",
		},
		{
			name:      "apply",
			id:        "run-12345",
			command:   "apply",
			status:    github.String("completed"),
			namespace: "default",
			workspace: "default",
			want:      "default/default | applied",
		},
	}

	for _, tt := range tests {
		testutil.Run(t, tt.name, func(t *testutil.T) {
			c := check{
				command:   tt.command,
				namespace: tt.namespace,
				workspace: tt.workspace,
				status:    tt.status,
				run:       tt.id,
			}

			if tt.out != "" {
				c.out, _ = ioutil.ReadFile(tt.out)
			}

			assert.Equal(t, tt.want, c.name())
		})
	}
}
