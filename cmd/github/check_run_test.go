package github

import (
	"io/ioutil"
	"testing"

	"github.com/leg100/etok/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunOutput(t *testing.T) {
	got, err := ioutil.ReadFile("fixtures/got.txt")
	require.NoError(t, err)

	t.Run("within maximum size", func(t *testing.T) {
		o := checkRun{maxFieldSize: defaultMaxFieldSize}

		_, err = o.Write(got)
		require.NoError(t, err)

		want, err := ioutil.ReadFile("fixtures/want.md")
		require.NoError(t, err)

		assert.Equal(t, string(want), o.details())
	})

	t.Run("exceeds maximum size", func(t *testing.T) {
		o := checkRun{
			// Default is 64k but we'll set to an artificially low number so
			// that we can easily test this maximum being breached
			maxFieldSize: 1000,
		}

		_, err = o.Write(got)
		require.NoError(t, err)

		want, err := ioutil.ReadFile("fixtures/want_truncated.md")
		require.NoError(t, err)

		assert.Equal(t, string(want), o.details())
	})

	t.Run("strip off refreshing lines", func(t *testing.T) {
		o := checkRun{
			stripRefreshing: true,
			maxFieldSize:    defaultMaxFieldSize,
		}

		_, err = o.Write(got)
		require.NoError(t, err)

		want, err := ioutil.ReadFile("fixtures/want_without_refresh.md")
		require.NoError(t, err)

		assert.Equal(t, string(want), o.details())
	})
}

func TestRunName(t *testing.T) {
	tests := []struct {
		name      string
		id        string
		command   string
		namespace string
		workspace string
		status    string
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
			want:      "default/default[plan] 12345",
		},
		{
			name:      "completed plan",
			id:        "run-12345",
			command:   "plan",
			status:    "completed",
			namespace: "default",
			workspace: "default",
			out:       "fixtures/plan.txt",
			want:      "default/default[+2~0-0] 12345",
		},
		{
			name:      "completed plan, no changes",
			id:        "run-12345",
			command:   "plan",
			status:    "completed",
			namespace: "default",
			workspace: "default",
			out:       "fixtures/plan_no_changes.txt",
			want:      "default/default[+0~0-0] 12345",
		},
		{
			name:      "apply",
			id:        "run-12345",
			command:   "apply",
			namespace: "default",
			workspace: "default",
			want:      "default/default[apply] 12345",
		},
	}

	for _, tt := range tests {
		testutil.Run(t, tt.name, func(t *testutil.T) {
			r := checkRun{
				command:   tt.command,
				namespace: tt.namespace,
				workspace: tt.workspace,
				status:    tt.status,
				id:        tt.id,
			}

			if tt.out != "" {
				r.out, _ = ioutil.ReadFile(tt.out)
			}

			assert.Equal(t, tt.want, r.name())
		})
	}
}
