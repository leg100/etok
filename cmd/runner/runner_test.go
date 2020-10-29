package runner

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/ioutil"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/kr/pty"
	"github.com/leg100/stok/cmd/envvars"
	"github.com/leg100/stok/pkg/app"
	"github.com/leg100/stok/pkg/archive"
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
			name: "magic string",
			args: []string{"--debug", "--", "sh", "-c", "echo -n hallelujah"},
			envs: map[string]string{
				"STOK_REQUIRE_MAGIC_STRING": "true",
				"STOK_DEBUG":                "true",
			},
			in:  bytes.NewBufferString("magicstring\n"),
			out: "hallelujah",
		},
		{
			name: "bad magic string",
			args: []string{"--", "sh", "-c", "echo -n this should never be printed"},
			envs: map[string]string{
				"STOK_REQUIRE_MAGIC_STRING": "true",
			},
			in:   bytes.NewBufferString("mag)J)Fring\n"),
			code: 1,
			err:  true,
		},
		{
			name: "EOF while waiting for magic string",
			args: []string{"--", "sh", "-c", "echo -n this should never be printed"},
			envs: map[string]string{
				"STOK_REQUIRE_MAGIC_STRING": "true",
			},
			in:   &eofReader{},
			code: 1,
			err:  true,
		},
		{
			name: "time out waiting for magic string",
			args: []string{"--", "sh", "-c", "echo -n this should never be printed"},
			envs: map[string]string{
				"STOK_REQUIRE_MAGIC_STRING": "true",
			},
			in: &delayedReader{time.Millisecond * 100},
			setOpts: func(o *RunnerOptions) {
				o.TimeoutClient = time.Millisecond * 20
			},
			code: 1,
			err:  true,
		},
	}

	for _, tt := range tests {
		testutil.Run(t, tt.name, func(t *testutil.T) {
			t.SetEnvs(tt.envs)

			out := new(bytes.Buffer)
			opts, err := app.NewFakeOpts(out, tt.objs...)
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
			cmd.Flags().BoolVar(&cmdOpts.Debug, "debug", true, "debug flag")
			log.SetLevel(log.DebugLevel)

			// Always run runner in unique temp dir
			cmdOpts.Path = t.NewTempDir().Chdir().Root()

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
	return len("magicstring"), nil
}
