package runner

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	kerrors "k8s.io/apimachinery/pkg/api/errors"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/creack/pty"
	"github.com/leg100/etok/cmd/envvars"
	cmdutil "github.com/leg100/etok/cmd/util"
	"github.com/leg100/etok/pkg/executor"
	"github.com/leg100/etok/pkg/globals"
	"github.com/leg100/etok/pkg/testobj"
	"github.com/leg100/etok/pkg/testutil"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh/terminal"
)

func TestRunnerCommand(t *testing.T) {
	testutil.Run(t, "shell command with args", func(t *testutil.T) {
		out, cmd, _ := setupRunnerCmd(t, "--", "echo foo")

		// Set flag via env var since that's how runner is invoked on a pod
		t.SetEnvs(map[string]string{
			"ETOK_COMMAND":   "sh",
			"ETOK_NAMESPACE": "foo",
		})

		envvars.SetFlagsFromEnvVariables(cmd)

		require.NoError(t, cmd.ExecuteContext(context.Background()))

		assert.Equal(t, "foo", strings.TrimSpace(out.String()))
	})

	testutil.Run(t, "shell command with non-zero exit", func(t *testutil.T) {
		_, cmd, _ := setupRunnerCmd(t, "--", "exit 101")

		// Set flag via env var since that's how runner is invoked on a pod
		t.SetEnvs(map[string]string{
			"ETOK_COMMAND":   "sh",
			"ETOK_NAMESPACE": "foo",
		})
		envvars.SetFlagsFromEnvVariables(cmd)

		// want exit code 101
		var exiterr *exec.ExitError
		if assert.True(t, errors.As(cmd.ExecuteContext(context.Background()), &exiterr)) {
			assert.Equal(t, 101, exiterr.ExitCode())
		}
	})

	testutil.Run(t, "terraform plan", func(t *testutil.T) {
		out, cmd, opts := setupRunnerCmd(t, "--", "-out", "plan.out")

		// Set flag via env var since that's how runner is invoked on a pod
		t.SetEnvs(map[string]string{
			"ETOK_COMMAND":   "plan",
			"ETOK_WORKSPACE": "foo",
			"ETOK_NAMESPACE": "foo",
		})
		envvars.SetFlagsFromEnvVariables(cmd)

		// Override executor with one that prints out cmd+args
		opts.exec = &executor.FakeExecutorEchoArgs{Out: out}

		require.NoError(t, cmd.ExecuteContext(context.Background()))

		want := "[terraform plan -out plan.out]"
		assert.Equal(t, want, strings.TrimSpace(out.String()))
	})

	testutil.Run(t, "terraform plan with custom namespace", func(t *testutil.T) {
		out, cmd, opts := setupRunnerCmd(t, "--", "-out", "plan.out")

		// Set flag via env var since that's how runner is invoked on a pod
		t.SetEnvs(map[string]string{
			"ETOK_COMMAND":   "plan",
			"ETOK_NAMESPACE": "dev",
			"ETOK_WORKSPACE": "foo",
		})
		envvars.SetFlagsFromEnvVariables(cmd)

		// Override executor with one that prints out cmd+args
		opts.exec = &executor.FakeExecutorEchoArgs{Out: out}

		require.NoError(t, cmd.ExecuteContext(context.Background()))

		want := "[terraform plan -out plan.out]"
		assert.Equal(t, want, strings.TrimSpace(out.String()))
	})

	testutil.Run(t, "terraform plan with new workspace", func(t *testutil.T) {
		out, cmd, opts := setupRunnerCmd(t, "--", "-out", "plan.out")

		// Set flag via env var since that's how runner is invoked on a pod
		t.SetEnvs(map[string]string{
			"ETOK_COMMAND":   "plan",
			"ETOK_WORKSPACE": "foo",
			"ETOK_NAMESPACE": "dev",
		})
		envvars.SetFlagsFromEnvVariables(cmd)

		// Override executor with one that prints out cmd+args
		opts.exec = &executor.FakeExecutorMissingWorkspace{Out: out}

		require.NoError(t, cmd.ExecuteContext(context.Background()))

		want := "[terraform plan -out plan.out]"
		assert.Equal(t, want, strings.TrimSpace(out.String()))
	})

	testutil.Run(t, "terraform apply", func(t *testutil.T) {
		out, cmd, opts := setupRunnerCmd(t, "--", "-auto-approve")

		// Set flag via env var since that's how runner is invoked on a pod
		t.SetEnvs(map[string]string{
			"ETOK_COMMAND":   "apply",
			"ETOK_WORKSPACE": "foo",
			"ETOK_NAMESPACE": "dev",
		})
		envvars.SetFlagsFromEnvVariables(cmd)

		// Override executor with one that prints out cmd+args
		opts.exec = &executor.FakeExecutorEchoArgs{Out: out}

		require.NoError(t, cmd.ExecuteContext(context.Background()))

		want := "[terraform apply -auto-approve]"
		assert.Equal(t, want, strings.TrimSpace(out.String()))
	})
}

func TestRunnerLockFile(t *testing.T) {
	testutil.Run(t, "with lock file", func(t *testutil.T) {
		out := new(bytes.Buffer)
		cmdOpts, err := cmdutil.NewFakeOpts(out, testobj.Run("dev", "run-12345", "init"))
		require.NoError(t, err)
		cmd, o := RunnerCmd(cmdOpts)
		cmd.SetOut(out)
		cmd.SetArgs([]string{"--", "true"})

		t.NewTempDir().Chdir().Write(globals.LockFile, []byte("plugin hashes"))

		// Set flag via env var since that's how runner is invoked on a pod
		t.SetEnvs(map[string]string{
			"ETOK_NAMESPACE": "dev",
			"ETOK_COMMAND":   "init",
			"ETOK_RUN_NAME":  "run-12345",
		})
		envvars.SetFlagsFromEnvVariables(cmd)

		assert.NoError(t, cmd.ExecuteContext(context.Background()))

		_, err = o.ConfigMapsClient(o.namespace).Get(context.Background(), "run-12345-lockfile", metav1.GetOptions{})
		assert.NoError(t, err)
	})

	testutil.Run(t, "without lock file", func(t *testutil.T) {
		_, cmd, o := setupRunnerCmd(t, "--", "true")

		// Set flag via env var since that's how runner is invoked on a pod
		t.SetEnvs(map[string]string{
			"ETOK_NAMESPACE": "dev",
			"ETOK_COMMAND":   "sh",
			"ETOK_RUN_NAME":  "run-12345",
		})
		envvars.SetFlagsFromEnvVariables(cmd)

		assert.NoError(t, cmd.ExecuteContext(context.Background()))

		_, err := o.ConfigMapsClient(o.namespace).Get(context.Background(), "run-12345-lockfile", metav1.GetOptions{})
		assert.True(t, kerrors.IsNotFound(err))
	})
}

func TestRunnerHandshake(t *testing.T) {
	tests := []struct {
		name string
		envs map[string]string
		err  error
		in   io.Reader
	}{
		{
			name: "handshake",
			envs: map[string]string{
				"ETOK_NAMESPACE": "dev",
				"ETOK_HANDSHAKE": "true",
				"ETOK_COMMAND":   "sh",
			},
			in: bytes.NewBufferString("opensesame\n"),
		},
		{
			name: "bad handshake",
			envs: map[string]string{
				"ETOK_NAMESPACE": "dev",
				"ETOK_HANDSHAKE": "true",
				"ETOK_COMMAND":   "sh",
			},
			in:  bytes.NewBufferString("mag)J)Fring\n"),
			err: errIncorrectHandshake,
		},
		{
			name: "time out waiting for handshake",
			envs: map[string]string{
				"ETOK_NAMESPACE":         "dev",
				"ETOK_HANDSHAKE":         "true",
				"ETOK_HANDSHAKE_TIMEOUT": "20ms",
				"ETOK_COMMAND":           "sh",
			},
			in:  &delayedReader{time.Millisecond * 100},
			err: errHandshakeTimeout,
		},
	}

	for _, tt := range tests {
		testutil.Run(t, "handshake", func(t *testutil.T) {
			_, cmd, opts := setupRunnerCmd(t)

			// Set flag via env var since that's how runner is invoked on a pod
			t.SetEnvs(tt.envs)
			envvars.SetFlagsFromEnvVariables(cmd)

			// Override executor with one that does a noop
			opts.exec = &executor.FakeExecutor{}

			// Create pseudoterminal to mimic TTY
			ptm, pts, err := pty.Open()
			require.NoError(t, err)
			opts.In = pts
			go func() {
				oldState, err := terminal.MakeRaw(int(ptm.Fd()))
				require.NoError(t, err)
				defer func() {
					_ = terminal.Restore(int(ptm.Fd()), oldState)
				}()
				// copy stdin to TTY
				io.Copy(ptm, tt.in)
			}()

			// Look for wanted error in returned error chain
			assert.True(t, errors.Is(cmd.ExecuteContext(context.Background()), tt.err))
		})
	}
}

func TestRunnerTarball(t *testing.T) {
	testutil.Run(t, "tarball", func(t *testutil.T) {
		// ls will check tarball extracted successfully and to the expected path
		_, cmd, _ := setupRunnerCmd(t, "--", "/bin/ls test1.tf")

		// Tarball path
		tarball := filepath.Join(t.NewTempDir().Root(), "archive.tar.gz")
		// Dest dir to extract tarball to
		dest := t.NewTempDir()
		// and the working dir (the pod the runner runs on will usually set
		// this)
		dest.Chdir()

		createTarballWithFiles(t, tarball, "test1.tf")

		// Set flag via env var since that's how runner is invoked on a pod
		t.SetEnvs(map[string]string{
			"ETOK_NAMESPACE": "dev",
			"ETOK_TARBALL":   tarball,
			"ETOK_COMMAND":   "sh",
			"ETOK_DEST":      dest.Root(),
		})
		envvars.SetFlagsFromEnvVariables(cmd)

		assert.NoError(t, cmd.ExecuteContext(context.Background()))
	})
}

func createTarballWithFiles(t *testutil.T, name string, filenames ...string) {
	f, err := os.Create(name)
	zw := gzip.NewWriter(f)
	tw := tar.NewWriter(zw)

	for _, fname := range filenames {
		err = tw.WriteHeader(&tar.Header{
			Name: fname,
			Mode: 0600,
		})
		require.NoError(t, err)
	}

	require.NoError(t, tw.Close())
	require.NoError(t, zw.Close())
	require.NoError(t, f.Close())
}

func setupRunnerCmd(t *testutil.T, args ...string) (*bytes.Buffer, *cobra.Command, *RunnerOptions) {
	out := new(bytes.Buffer)
	o, err := cmdutil.NewFakeOpts(out)
	require.NoError(t, err)
	cmd, cmdOpts := RunnerCmd(o)
	cmd.SetOut(out)
	cmd.SetArgs(args)

	cmdOpts.dest = t.NewTempDir().Chdir().Root()

	return out, cmd, cmdOpts
}

// delayedReader mocks reader that only returns read call after delay
type delayedReader struct {
	delay time.Duration
}

func (e *delayedReader) Read(p []byte) (int, error) {
	time.Sleep(e.delay)
	return len("opensesame"), nil
}
