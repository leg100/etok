package e2e

import (
	"bytes"
	goctx "context"
	"io"
	"os"
	"os/exec"
	"regexp"
	"testing"
	"time"

	"cloud.google.com/go/storage"

	"github.com/kr/pty"
	"golang.org/x/sys/unix"

	framework "github.com/operator-framework/operator-sdk/pkg/test"
	"github.com/operator-framework/operator-sdk/pkg/test/e2eutil"
)

var (
	retryInterval        = time.Second * 5
	timeout              = time.Second * 60
	cleanupRetryInterval = time.Second * 1
	cleanupTimeout       = time.Second * 5
)

const (
	buildPath     = "../../../build/_output/bin/stok"
	workspaceName = "stok"
	workspacePath = "./test/e2e/workspace"
	backendBucket = "master-anagram-224816-tfstate"
)

func TestStok(t *testing.T) {
	ctx := framework.NewTestCtx(t)
	defer ctx.Cleanup()

	// get namespace
	namespace, err := ctx.GetOperatorNamespace()
	if err != nil {
		t.Fatal(err)
	}
	// get global framework variables
	f := framework.Global
	// wait for stok-operator to be ready
	err = e2eutil.WaitForOperatorDeployment(t, f.KubeClient, namespace, "stok-operator", 1, time.Second*5, time.Second*30)
	if err != nil {
		t.Fatal(err)
	}

	// we want a clean backend beforehand :)
	sclient, err := storage.NewClient(goctx.Background())
	if err != nil {
		t.Fatal(err)
	}
	bkt := sclient.Bucket(backendBucket)
	// ignore errors
	bkt.Object("default.tfstate").Delete(goctx.Background())
	bkt.Object("default.tflock").Delete(goctx.Background())

	tests := []struct {
		name            string
		args            []string
		path            string
		wantExitCode    int
		wantStdoutRegex *regexp.Regexp
		pty             bool
		wantWarnings    []string
		stdin           []byte
		testQueuing     bool
	}{
		{
			name:            "stok",
			args:            []string{},
			wantExitCode:    0,
			wantStdoutRegex: regexp.MustCompile(`^Supercharge terraform on kubernetes`),
			pty:             false,
		},
		{
			name:            "stok init",
			args:            []string{"init", "--", "-no-color", "-input=false"},
			wantExitCode:    0,
			wantStdoutRegex: regexp.MustCompile(`\nInitializing the backend`),
			pty:             false,
			wantWarnings:    []string{"Unable to use a TTY - input is not a terminal or the right kind of file", "Failed to attach to pod TTY; falling back to streaming logs"},
		},
		{
			name:            "stok plan",
			args:            []string{"plan", "--", "-no-color", "-input=false", "-var 'suffix=foo'"},
			wantExitCode:    0,
			wantStdoutRegex: regexp.MustCompile(`Refreshing Terraform state in-memory prior to plan`),
			pty:             false,
			wantWarnings:    []string{"Unable to use a TTY - input is not a terminal or the right kind of file", "Failed to attach to pod TTY; falling back to streaming logs"},
		},
		{
			name:            "stok plan with pty",
			args:            []string{"plan", "--", "-no-color", "-input=true"},
			wantExitCode:    0,
			wantStdoutRegex: regexp.MustCompile(`(?s)var\.suffix.*Enter a value:.*Refreshing Terraform state in-memory prior to plan`),
			pty:             true,
			stdin:           []byte("foo\n"),
		},
		{
			name:            "stok apply with pty",
			args:            []string{"apply", "--", "-no-color", "-input=true"},
			wantExitCode:    0,
			wantStdoutRegex: regexp.MustCompile(`Apply complete! Resources: 1 added, 0 changed, 0 destroyed.`),
			pty:             true,
			stdin:           []byte("foo\nyes\n"),
		},
		{
			name:            "stok shell",
			args:            []string{"shell"},
			wantExitCode:    0,
			wantStdoutRegex: regexp.MustCompile(`Linux`),
			pty:             true,
			stdin:           []byte("uname; sleep 1; exit\n"),
		},
		{
			name:            "stok queuing",
			args:            []string{"shell", "--", "\"uname; sleep 5\""},
			wantExitCode:    0,
			wantStdoutRegex: regexp.MustCompile(`Linux`),
			pty:             false,
			testQueuing:     true,
		},
	}

	// invoke stok with each test case
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command(buildPath, tt.args...)
			cmd.Dir = workspacePath
			cmd.Env = envVars(namespace)

			outbuf := new(bytes.Buffer)
			out := io.MultiWriter(outbuf, os.Stdout)

			errbuf := new(bytes.Buffer)
			stderr := io.MultiWriter(errbuf, os.Stderr)

			if tt.pty {
				terminal, err := pty.Start(cmd)
				if err != nil {
					t.Fatal(err)
				}
				defer terminal.Close()

				// https://github.com/creack/pty/issues/82#issuecomment-502785533
				echoOff(terminal)
				stdinR, stdinW := io.Pipe()
				go io.Copy(terminal, stdinR)
				stdinW.Write(tt.stdin)

				// ... and the pty to stdout.
				_, _ = io.Copy(out, terminal)
			} else {
				// without pty, so just use a buffer, and skip stdin
				cmd.Stdout = out
				cmd.Stderr = stderr

				if err = cmd.Start(); err != nil {
					t.Fatal(err)
				}
			}

			if tt.testQueuing {
				testQueuing(t, tt.args, tt.wantExitCode, namespace)
			}

			exitCodeTest(t, cmd.Wait(), tt.wantExitCode)

			// Without a pty we expect a warning log msg telling us as much.
			// (We can use stderr without pty but not with pty)
			if !tt.pty {
				got := errbuf.String()
				for _, want := range tt.wantWarnings {
					if !regexp.MustCompile(want).MatchString(got) {
						t.Errorf("want '%s', got '%s'\n", want, got)
					}
				}
			}

			got := outbuf.String()
			if !tt.wantStdoutRegex.MatchString(got) {
				t.Errorf("expected stdout to match '%s' but got '%s'\n", tt.wantStdoutRegex, got)
			}
		})
	}
}

func envVars(namespace string) []string {
	return append(os.Environ(), "STOK_NAMESPACE="+namespace, "STOK_WORKSPACE="+workspaceName)
}

// Launch the command a 2nd time, in parallel, one second later, to test queuing functionality
func testQueuing(t *testing.T, args []string, wantExitCode int, namespace string) {
	cmd := exec.Command(buildPath, args...)
	cmd.Dir = workspacePath
	cmd.Env = envVars(namespace)

	outbuf := new(bytes.Buffer)
	out := io.MultiWriter(outbuf, os.Stdout)
	cmd.Stdout = out

	time.Sleep(time.Second)

	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}

	exitCodeTest(t, cmd.Wait(), wantExitCode)

	got := outbuf.String()
	want := `Waiting in workspace queue; position: 1`
	if !regexp.MustCompile(want).MatchString(got) {
		t.Errorf("want '%s', got '%s'\n", want, got)
	}
}

func echoOff(f *os.File) {
	fd := int(f.Fd())
	//      const ioctlReadTermios = unix.TIOCGETA // OSX.
	const ioctlReadTermios = unix.TCGETS // Linux
	//      const ioctlWriterTermios =  unix.TIOCSETA // OSX.
	const ioctlWriteTermios = unix.TCSETS // Linux

	termios, err := unix.IoctlGetTermios(fd, ioctlReadTermios)
	if err != nil {
		panic(err)
	}

	newState := *termios
	newState.Lflag &^= unix.ECHO
	newState.Lflag |= unix.ICANON | unix.ISIG
	newState.Iflag |= unix.ICRNL
	if err := unix.IoctlSetTermios(fd, ioctlWriteTermios, &newState); err != nil {
		panic(err)
	}
}

func exitCodeTest(t *testing.T, err error, wantExitCode int) {
	if exiterr, ok := err.(*exec.ExitError); ok {
		if wantExitCode != exiterr.ExitCode() {
			t.Errorf("expected exit code %d, got %d\n", wantExitCode, exiterr.ExitCode())
		}
	} else if err != nil {
		t.Error(err)
	} else {
		// got exit code 0 and no error
		if wantExitCode != 0 {
			t.Errorf("expected exit code %d, got 0\n", wantExitCode)
		}
	}
}
