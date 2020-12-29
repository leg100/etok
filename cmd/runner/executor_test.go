package runner

import (
	"bytes"
	"context"
	"errors"
	osexec "os/exec"
	"path/filepath"
	"testing"

	cmdutil "github.com/leg100/etok/cmd/util"
	"github.com/leg100/etok/pkg/testutil"
	"github.com/stretchr/testify/assert"
)

func TestExecutor(t *testing.T) {
	testutil.Run(t, "stdout", func(t *testutil.T) {
		out := new(bytes.Buffer)

		exec := &Exec{IOStreams: cmdutil.IOStreams{Out: out}}
		exec.Execute(context.Background(), []string{"echo", "-n", "plan"})

		assert.Equal(t, "plan", out.String())
	})

	testutil.Run(t, "stdin", func(t *testutil.T) {
		out := new(bytes.Buffer)

		exec := &Exec{IOStreams: cmdutil.IOStreams{In: bytes.NewBufferString("input"), Out: out}}
		exec.Execute(context.Background(), []string{"cat"})

		assert.Equal(t, "input", out.String())
	})

	testutil.Run(t, "output to disk", func(t *testutil.T) {
		path := t.NewTempDir()

		(&Exec{}).Execute(context.Background(), []string{"touch", "a.file"}, withPath(path.Root()))

		assert.FileExists(t, filepath.Join(path.Root(), "a.file"))
	})

	testutil.Run(t, "non-zero exit", func(t *testutil.T) {
		err := (&Exec{}).Execute(context.Background(), []string{"sh", "-c", "exit 101"})

		// want exit code 101
		var exiterr *osexec.ExitError
		if assert.True(t, errors.As(err, &exiterr)) {
			assert.Equal(t, 101, exiterr.ExitCode())
		}
	})
}
