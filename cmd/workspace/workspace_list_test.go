package workspace

import (
	"bytes"
	"context"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	cmdutil "github.com/leg100/etok/cmd/util"
	"github.com/leg100/etok/pkg/env"
	"github.com/leg100/etok/pkg/testobj"
	"github.com/leg100/etok/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestListWorkspaces(t *testing.T) {
	tests := []struct {
		name string
		objs []runtime.Object
		args []string
		env  *env.Env
		err  bool
		out  string
	}{
		{
			name: "WithEnvironmentFile",
			objs: []runtime.Object{
				testobj.Workspace("default", "workspace-1", testobj.WithRepository("https://github.com/leg100/etok.git"), testobj.WithWorkingDir(".")),
				testobj.Workspace("dev", "workspace-2", testobj.WithRepository("https://github.com/leg100/etok.git"), testobj.WithWorkingDir(".")),
			},
			args: []string{},
			env:  &env.Env{Namespace: "default", Workspace: "workspace-1"},
			out:  "*\tdefault/workspace-1\n\tdev/workspace-2\n",
		},
		{
			name: "WithoutEnvironmentFile",
			objs: []runtime.Object{
				testobj.Workspace("default", "workspace-1", testobj.WithRepository("https://github.com/leg100/etok.git"), testobj.WithWorkingDir(".")),
				testobj.Workspace("dev", "workspace-2", testobj.WithRepository("https://github.com/leg100/etok.git"), testobj.WithWorkingDir(".")),
			},
			args: []string{},
			out:  "\tdefault/workspace-1\n\tdev/workspace-2\n",
		},
	}
	for _, tt := range tests {
		testutil.Run(t, tt.name, func(t *testutil.T) {
			path := t.NewTempDir().Chdir().Root()

			// Make the path a git repo
			repo, err := git.PlainInit(path, false)
			require.NoError(t, err)

			// Set a remote so that we can check that the code successfully sets
			// the remote URL in the workspace spec
			_, err = repo.CreateRemote(&config.RemoteConfig{
				Name: "origin",
				URLs: []string{"git@github.com:leg100/etok.git"},
			})

			// Write .terraform/environment
			if tt.env != nil {
				require.NoError(t, tt.env.Write(path))
			}

			out := new(bytes.Buffer)

			f := cmdutil.NewFakeFactory(out, tt.objs...)

			cmd := listCmd(f)
			cmd.SetArgs(tt.args)
			cmd.SetOut(f.Out)

			t.CheckError(tt.err, cmd.ExecuteContext(context.Background()))

			assert.Equal(t, tt.out, out.String())
		})
	}
}
