package runner

import (
	"bytes"
	"context"
	"errors"
	"io/ioutil"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/leg100/stok/api/stok.goalspike.com/v1alpha1"
	"github.com/leg100/stok/pkg/app"
	"github.com/leg100/stok/pkg/archive"
	"github.com/leg100/stok/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestRunner(t *testing.T) {
	tests := []struct {
		name       string
		setOpts    func(opts *app.Options)
		out        string
		assertions func(opts *app.Options)
		err        bool
		code       int
	}{
		{
			name: "defaults",
			setOpts: func(opts *app.Options) {
				opts.Args = []string{"sh", "-c", "echo -n hallelujah"}
			},
			out: "hallelujah",
		},
		{
			name: "defaults + tarball",
			setOpts: func(opts *app.Options) {
				opts.Tarball = "archive.tar.gz"
				opts.Args = []string{"/bin/ls", "test1.tf"}
			},
			out: "test1.tf\n",
		},
		{
			name: "defaults + workspace",
			setOpts: func(opts *app.Options) {
				opts.Kind = "Workspace"
				opts.Args = []string{"sh", "-c", "echo -n hallelujah"}
			},
			out: "hallelujah",
		},
		{
			name: "non-zero exit code",
			setOpts: func(opts *app.Options) {
				opts.Args = []string{"sh", "-c", "echo -n alienation; exit 2"}
			},
			out:  "alienation",
			code: 2,
			err:  true,
		},
	}

	for _, tt := range tests {
		testutil.Run(t, tt.name, func(t *testutil.T) {
			out := new(bytes.Buffer)
			opts, err := app.NewFakeOptsWithClients(out)
			require.NoError(t, err)

			if tt.setOpts != nil {
				tt.setOpts(opts)
			}

			// Change into tmpdir and set as path
			opts.Path = t.NewTempDir().Chdir().Root()

			if opts.Tarball != "" {
				// Create dummy tarball
				createTarballWithFiles(t, opts.Tarball, "test1.tf")
			}

			// Create resource it watches (w/o wait annotation)
			switch opts.Kind {
			case "Run":
				opts.StokClient().
					StokV1alpha1().
					Runs(opts.Namespace).
					Create(context.Background(), testRun(opts.Namespace, opts.Name), metav1.CreateOptions{})
			case "Workspace":
				opts.StokClient().
					StokV1alpha1().
					Workspaces(opts.Namespace).
					Create(context.Background(), testWorkspace(opts.Namespace, opts.Name), metav1.CreateOptions{})
			}

			err = (&Runner{Options: opts}).Run(context.Background())
			t.CheckError(tt.err, err)

			// Check exit code
			assert.Equal(t, tt.code, unwrapCode(err))

			assert.Equal(t, tt.out, out.String())

			if tt.assertions != nil {
				tt.assertions(opts)
			}
		})
	}
}

func createTarballWithFiles(t *testutil.T, name string, filenames ...string) {
	path := t.NewTempDir().Root()

	// Create dummy zero-sized files to be included in archive
	for _, f := range filenames {
		fpath := filepath.Join(path, f)
		ioutil.WriteFile(fpath, []byte{}, 0644)
	}

	// Create test tarball
	tar, err := archive.Create(path)
	require.NoError(t, err)

	// Write tarball to current path
	err = ioutil.WriteFile(name, tar, 0644)
	require.NoError(t, err)
}

func testRun(namespace, name string) *v1alpha1.Run {
	return &v1alpha1.Run{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

func testWorkspace(namespace, name string) *v1alpha1.Workspace {
	return &v1alpha1.Workspace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

// Unwrap exit code from error message
func unwrapCode(err error) int {
	if err != nil {
		var exiterr *exec.ExitError
		if errors.As(err, &exiterr) {
			return exiterr.ExitCode()
		}
		return 1
	}
	return 0
}
