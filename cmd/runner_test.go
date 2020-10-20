package cmd

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/leg100/stok/pkg/app"
	"github.com/leg100/stok/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestRunner(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		envs       map[string]string
		objs       []runtime.Object
		err        bool
		assertions func(opts *app.Options)
	}{
		{
			name: "explicit env vars",
			args: []string{"runner", "--", "/bin/ls", "test1.tf"},
			envs: map[string]string{
				"STOK_KIND":      "Run",
				"STOK_NAME":      "foo",
				"STOK_NAMESPACE": "default",
				"STOK_TIMEOUT":   "10s",
				"STOK_TARBALL":   "doesnotexist.tar.gz",
			},
			assertions: func(opts *app.Options) {
				assert.Equal(t, "Run", opts.Kind)
				assert.Equal(t, "foo", opts.Name)
				assert.Equal(t, "default", opts.Namespace)
				assert.Equal(t, time.Second*10, opts.TimeoutClient)
				assert.Equal(t, "doesnotexist.tar.gz", opts.Tarball)
				assert.Equal(t, ".", opts.Path)
				assert.Equal(t, []string{"/bin/ls", "test1.tf"}, opts.Args)
			},
		},
		{
			name: "implicit defaults",
			args: []string{"runner", "--", "/bin/ls", "test1.tf"},
			envs: map[string]string{
				"STOK_KIND": "Run",
				"STOK_NAME": "foo",
			},
			assertions: func(opts *app.Options) {
				assert.Equal(t, "Run", opts.Kind)
				assert.Equal(t, "foo", opts.Name)
				assert.Equal(t, "default", opts.Namespace)
				assert.Equal(t, time.Second*10, opts.TimeoutClient)
				assert.Equal(t, "", opts.Tarball)
				assert.Equal(t, ".", opts.Path)
				assert.Equal(t, []string{"/bin/ls", "test1.tf"}, opts.Args)
			},
		},
		{
			name: "ensure kube clients are created",
			args: []string{"runner"},
			assertions: func(opts *app.Options) {
				assert.NotNil(t, opts.KubeClient())
				assert.NotNil(t, opts.StokClient())
			},
		},
	}

	for _, tt := range tests {
		testutil.Run(t, tt.name, func(t *testutil.T) {
			t.NewTempDir().Chdir()

			t.SetEnvs(tt.envs)

			opts, err := app.NewFakeOpts(new(bytes.Buffer), tt.objs...)
			require.NoError(t, err)

			t.CheckError(tt.err, ParseArgs(context.Background(), tt.args, opts))

			tt.assertions(opts)
		})
	}
}
