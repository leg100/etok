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
	"testing"
	"time"

	"github.com/creack/pty"
	"github.com/leg100/stok/cmd/envvars"
	cmdutil "github.com/leg100/stok/cmd/util"
	"github.com/leg100/stok/pkg/log"
	"github.com/leg100/stok/testutil"
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
		err           bool
		createTarball func(*testutil.T, string)
		setOpts       func(*RunnerOptions)
		assertions    func(*RunnerOptions)
		in            io.Reader
		out           string
		code          int
	}{
		{
			name: "no args",
			err:  true,
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
			code: 2,
			err:  true,
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
			in:   bytes.NewBufferString("mag)J)Fring\n"),
			code: 1,
			err:  true,
		},
		{
			name: "EOF while waiting for handshake",
			args: []string{"--", "sh", "-c", "echo -n this should never be printed"},
			envs: map[string]string{
				"STOK_HANDSHAKE": "true",
			},
			in:   &eofReader{},
			code: 1,
			err:  true,
		},
		{
			name: "time out waiting for handshake",
			args: []string{"--", "sh", "-c", "echo -n this should never be printed"},
			envs: map[string]string{
				"STOK_HANDSHAKE":         "true",
				"STOK_HANDSHAKE_TIMEOUT": "20ms",
			},
			in:   &delayedReader{time.Millisecond * 100},
			code: 1,
			err:  true,
		},
	}

	for _, tt := range tests {
		testutil.Run(t, tt.name, func(t *testutil.T) {
			t.SetEnvs(tt.envs)

			out := new(bytes.Buffer)
			opts, err := cmdutil.NewFakeOpts(out, tt.objs...)
			require.NoError(t, err)

			if tt.in != nil {
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

			if tt.setOpts != nil {
				tt.setOpts(cmdOpts)
			}

			if tt.envs != nil {
				envvars.SetFlagsFromEnvVariables(cmd)
			}

			t.CheckError(tt.err, cmd.ExecuteContext(context.Background()))

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

// eofReader mocks terminal sending Ctrl-D
type eofReader struct{}

func (e *eofReader) Read(p []byte) (int, error) {
	p[0] = 0x4
	return 1, nil
}

// delayedReader mocks reader that only returns read call after delay
type delayedReader struct {
	delay time.Duration
}

func (e *delayedReader) Read(p []byte) (int, error) {
	time.Sleep(e.delay)
	return len("opensesame"), nil
}
