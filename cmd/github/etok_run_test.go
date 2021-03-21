package github

import (
	"io/ioutil"
	"testing"

	"github.com/leg100/etok/api/etok.dev/v1alpha1"
	"github.com/leg100/etok/pkg/testobj"
	"github.com/leg100/etok/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEtokRunOutput(t *testing.T) {
	got, err := ioutil.ReadFile("fixtures/got.txt")
	require.NoError(t, err)

	t.Run("within maximum size", func(t *testing.T) {
		o := etokRun{maxFieldSize: defaultMaxFieldSize}

		_, err = o.Write(got)
		require.NoError(t, err)

		want, err := ioutil.ReadFile("fixtures/want.md")
		require.NoError(t, err)

		assert.Equal(t, string(want), o.details())
	})

	t.Run("exceeds maximum size", func(t *testing.T) {
		o := etokRun{
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
		o := etokRun{
			etokAppOptions: etokAppOptions{stripRefreshing: true},
			maxFieldSize:   defaultMaxFieldSize,
		}

		_, err = o.Write(got)
		require.NoError(t, err)

		want, err := ioutil.ReadFile("fixtures/want_without_refresh.md")
		require.NoError(t, err)

		assert.Equal(t, string(want), o.details())
	})
}

func TestEtokRunName(t *testing.T) {
	tests := []struct {
		name      string
		id        string
		command   string
		workspace *v1alpha1.Workspace
		completed bool
		want      string
		// Path to fixture from which to populate output buffer
		out string
	}{
		{
			name:      "incomplete plan",
			id:        "run-12345",
			command:   "plan",
			workspace: testobj.Workspace("default", "default", testobj.WithRepository("bob/myrepo"), testobj.WithBranch("changes"), testobj.WithWorkingDir("subdir")),
			want:      "default/default[plan] 12345",
		},
		{
			name:      "completed plan",
			id:        "run-12345",
			command:   "plan",
			completed: true,
			workspace: testobj.Workspace("default", "default", testobj.WithRepository("bob/myrepo"), testobj.WithBranch("changes"), testobj.WithWorkingDir("subdir")),
			out:       "fixtures/plan.txt",
			want:      "default/default[+2~0-0] 12345",
		},
		{
			name:      "completed plan, no changes",
			id:        "run-12345",
			command:   "plan",
			completed: true,
			workspace: testobj.Workspace("default", "default", testobj.WithRepository("bob/myrepo"), testobj.WithBranch("changes"), testobj.WithWorkingDir("subdir")),
			out:       "fixtures/plan_no_changes.txt",
			want:      "default/default[+0~0-0] 12345",
		},
		{
			name:      "apply",
			id:        "run-12345",
			command:   "apply",
			workspace: testobj.Workspace("default", "default", testobj.WithRepository("bob/myrepo"), testobj.WithBranch("changes"), testobj.WithWorkingDir("subdir")),
			want:      "default/default[apply] 12345",
		},
	}

	for _, tt := range tests {
		testutil.Run(t, tt.name, func(t *testutil.T) {
			r := etokRun{
				command:   tt.command,
				completed: tt.completed,
				id:        tt.id,
				workspace: tt.workspace,
			}

			if tt.out != "" {
				r.out, _ = ioutil.ReadFile(tt.out)
			}

			assert.Equal(t, tt.want, r.name())
		})
	}
}

func TestLauncherArgs(t *testing.T) {
	tests := []struct {
		name     string
		id       string
		command  string
		previous string
		want     []string
	}{
		{
			name:     "default",
			id:       "run-12345",
			command:  "plan",
			previous: "",
			want:     []string{"-c", "set -e\n\nterraform init -no-color -input=false\n\n\nterraform plan -no-color -input=false -out=/plans/run-12345\n\n"},
		},
		{
			name:     "default",
			id:       "run-12345",
			command:  "apply",
			previous: "run-12345",
			want:     []string{"-c", "set -e\n\nterraform init -no-color -input=false\n\n\n\nterraform apply -no-color -input=false /plans/run-12345\n"},
		},
	}

	for _, tt := range tests {
		testutil.Run(t, tt.name, func(t *testutil.T) {
			args, err := launcherArgs(tt.id, tt.command, tt.previous)
			require.NoError(t, err)
			assert.Equal(t, tt.want, args)
		})
	}
}
