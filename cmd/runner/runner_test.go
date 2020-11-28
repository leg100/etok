package runner

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/creack/pty"
	"github.com/leg100/stok/cmd/envvars"
	cmdutil "github.com/leg100/stok/cmd/util"
	"github.com/leg100/stok/pkg/log"
	"github.com/leg100/stok/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh/terminal"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestRunner(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		envs          map[string]string
		objs          []runtime.Object
		err           func(*testutil.T, error)
		createTarball func(*testutil.T, string)
		assertions    func(*RunnerOptions)
		in            io.Reader
		out           string
	}{
		{
			name: "no args",
			err: func(t *testutil.T, err error) {
				assert.Equal(t, err.Error(), "requires at least 1 arg(s), only received 0")
			},
		},
		{
			name: "defaults",
			args: []string{"--", "sh", "-c", "echo -n hallelujah"},
			out:  "hallelujah",
		},
		{
			name: "extract archive",
			args: []string{"--", "/bin/ls", "test1.tf"},
			envs: map[string]string{
				"STOK_TARBALL": "archive.tar.gz",
			},
			createTarball: func(t *testutil.T, path string) {
				createTarballWithFiles(t, filepath.Join(path, "archive.tar.gz"), "test1.tf")
			},
			out: "test1.tf\n",
		},
		{
			name: "non-zero exit code",
			args: []string{"--", "sh", "-c", "echo -n alienation; exit 2"},
			out:  "alienation",
			err: func(t *testutil.T, err error) {
				// want exit code 2
				var exiterr *exec.ExitError
				if assert.True(t, errors.As(err, &exiterr)) {
					assert.Equal(t, 2, exiterr.ExitCode())
				}
			},
		},
		{
			name: "handshake",
			args: []string{"--debug", "--", "sh", "-c", "echo -n hallelujah"},
			envs: map[string]string{
				"STOK_HANDSHAKE": "true",
				"STOK_DEBUG":     "true",
			},
			in:  bytes.NewBufferString("opensesame\n"),
			out: "hallelujah",
		},
		{
			name: "bad handshake",
			args: []string{"--", "sh", "-c", "echo -n this should never be printed"},
			envs: map[string]string{
				"STOK_HANDSHAKE": "true",
			},
			in: bytes.NewBufferString("mag)J)Fring\n"),
			err: func(t *testutil.T, err error) {
				assert.EqualError(t, err, fmt.Sprintf("[runner] handshake: expected '%s' but received: %s", cmdutil.HandshakeString, "mag)J)Frin"))
			},
		},
		{
			name: "time out waiting for handshake",
			args: []string{"--", "sh", "-c", "echo -n this should never be printed"},
			envs: map[string]string{
				"STOK_HANDSHAKE":         "true",
				"STOK_HANDSHAKE_TIMEOUT": "20ms",
			},
			in: &delayedReader{time.Millisecond * 100},
			err: func(t *testutil.T, err error) {
				assert.EqualError(t, err, "[runner] timed out waiting for handshake")
			},
		},
	}

	for _, tt := range tests {
		testutil.Run(t, tt.name, func(t *testutil.T) {
			t.SetEnvs(tt.envs)

			out := new(bytes.Buffer)
			opts, err := cmdutil.NewFakeOpts(out, tt.objs...)
			require.NoError(t, err)

			cmd, cmdOpts := RunnerCmd(opts)
			cmd.SetOut(out)
			cmd.SetArgs(tt.args)

			// Set debug flag (that root cmd otherwise sets)
			cmd.Flags().BoolVar(&cmdOpts.Debug, "debug", false, "debug flag")
			log.SetLevel(log.DebugLevel)

			// Always run runner in unique temp dir
			cmdOpts.Path = t.NewTempDir().Chdir().Root()
			cmdOpts.Dest = cmdOpts.Path

			if tt.createTarball != nil {
				tt.createTarball(t, cmdOpts.Path)
			}

			if tt.envs != nil {
				envvars.SetFlagsFromEnvVariables(cmd)
			}

			// If handshake is enabled, mimic a TTY
			if cmdOpts.Handshake {
				// create pseudoterminal to mimic TTY
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
			}

			err = cmd.ExecuteContext(context.Background())
			if tt.err != nil {
				tt.err(t, err)
			} else {
				assert.NoError(t, err)
			}

			if tt.assertions != nil {
				tt.assertions(cmdOpts)
			}
		})
	}
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

// delayedReader mocks reader that only returns read call after delay
type delayedReader struct {
	delay time.Duration
}

func (e *delayedReader) Read(p []byte) (int, error) {
	time.Sleep(e.delay)
	return len("opensesame"), nil
}
