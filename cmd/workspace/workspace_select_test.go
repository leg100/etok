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

func TestWorkspaceSelect(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantEnv *env.Env
		objs    []runtime.Object
		out     string
		err     bool
	}{
		{
			name:    "defaults",
			args:    []string{"networking"},
			wantEnv: &env.Env{Namespace: "default", Workspace: "networking"},
			objs: []runtime.Object{
				testobj.Workspace("default", "networking", testobj.WithWorkingDir("."), testobj.WithRepository("https://github.com/leg100/etok.git")),
			},
			out: "Current workspace now: default/networking\n",
		},
		{
			name:    "with explicit namespace",
			args:    []string{"networking", "--namespace", "dev"},
			wantEnv: &env.Env{Namespace: "dev", Workspace: "networking"},
			objs: []runtime.Object{
				testobj.Workspace("dev", "networking", testobj.WithWorkingDir("."), testobj.WithRepository("https://github.com/leg100/etok.git")),
			},
			out: "Current workspace now: dev/networking\n",
		},
	}

	for _, tt := range tests {
		testutil.Run(t, tt.name, func(t *testutil.T) {
			out := new(bytes.Buffer)
			f := cmdutil.NewFakeFactory(out, tt.objs...)

			path := t.NewTempDir().Chdir().Root()

			// Make the path a git repo
			repo, err := git.PlainInit(path, false)
			require.NoError(t, err)

			// Set a remote so that we can check that the code successfully
			// sets the remote URL in the workspace spec
			_, err = repo.CreateRemote(&config.RemoteConfig{
				Name: "origin",
				URLs: []string{"git@github.com:leg100/etok.git"},
			})

			cmd := selectCmd(f)
			cmd.SetArgs(tt.args)
			cmd.SetOut(f.Out)

			t.CheckError(tt.err, cmd.ExecuteContext(context.Background()))

			assert.Equal(t, tt.out, out.String())

			// Confirm .terraform/environment was written with expected contents
			etokenv, err := env.Read(path)
			require.NoError(t, err)
			assert.Equal(t, tt.wantEnv, etokenv)
		})
	}
}
