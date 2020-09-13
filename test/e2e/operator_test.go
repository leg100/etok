package e2e

import (
	"bytes"
	goctx "context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"testing"

	"cloud.google.com/go/storage"

	"github.com/kr/pty"
	"golang.org/x/sys/unix"
)

const (
	buildPath     = "../../../stok"
	workspacePath = "./workspace"
	backendBucket = "automatize-tfstate"
	backendPrefix = "e2e"

	// Namespace in which stok workspace will be created in,
	// and commands tested in
	wsNamespace = "default"

	// Name of workspace to be created
	wsName = "foo"

	// Name of second workspace to be created
	wsName2 = "bar"
)

var kubectx = flag.String("context", "kind-kind", "Kubeconfig context to use for tests")

// End-to-end tests
func TestStok(t *testing.T) {
	fmt.Printf("Kubernetes context set to: %s\n", *kubectx)

	// we want a clean backend beforehand
	sclient, err := storage.NewClient(goctx.Background())
	if err != nil {
		t.Fatal(err)
	}
	bkt := sclient.Bucket(backendBucket)
	// ignore errors
	bkt.Object(backendPrefix + "/default.tfstate").Delete(goctx.Background())
	bkt.Object(backendPrefix + "/default.tflock").Delete(goctx.Background())

	tests := []struct {
		name            string
		args            []string
		path            string
		wantExitCode    int
		wantStdoutRegex *regexp.Regexp
		pty             bool
		wantWarnings    []string
		stdin           []byte
		queueAdditional int
	}{
		{
			name:            "stok",
			args:            []string{},
			wantExitCode:    0,
			wantStdoutRegex: regexp.MustCompile(`^Supercharge terraform on kubernetes`),
			pty:             false,
		},
		{
			name:            "stok version",
			args:            []string{"-v"},
			wantExitCode:    0,
			wantStdoutRegex: regexp.MustCompile(`^stok version v.+\t[a-f0-9]+`),
			pty:             false,
		},
		{
			name:            "new workspace",
			args:            []string{"workspace", "new", wsName, "--timeout", "5s", "--timeout-pod", "60s", "--context", *kubectx, "--backend-type", "gcs", "--backend-config", "bucket=automatize-tfstate,prefix=e2e"},
			wantExitCode:    0,
			wantStdoutRegex: regexp.MustCompile(``),
			pty:             false,
		},
		{
			name:            "second new workspace",
			args:            []string{"workspace", "new", wsName2, "--timeout", "5s", "--timeout-pod", "60s", "--context", *kubectx},
			wantExitCode:    0,
			wantStdoutRegex: regexp.MustCompile(``),
			pty:             false,
		},
		{
			name:            "list workspaces",
			args:            []string{"workspace", "list", "--context", *kubectx},
			wantExitCode:    0,
			wantStdoutRegex: regexp.MustCompile(fmt.Sprintf("\\*\t%s/%s\n\t%s/%s", wsNamespace, wsName2, wsNamespace, wsName)),
			pty:             false,
		},
		{
			name:            "select first workspace",
			args:            []string{"workspace", "select", wsName},
			wantExitCode:    0,
			wantStdoutRegex: regexp.MustCompile(``),
			pty:             false,
		},
		{
			name:            "show current workspace",
			args:            []string{"workspace", "show"},
			wantExitCode:    0,
			wantStdoutRegex: regexp.MustCompile(wsNamespace + "/" + wsName),
			pty:             false,
		},
		{
			name:            "stok init",
			args:            []string{"init", "--context", *kubectx, "--", "-no-color", "-input=false"},
			wantExitCode:    0,
			wantStdoutRegex: regexp.MustCompile(`Initializing the backend`),
			pty:             false,
			wantWarnings:    []string{"Unable to use a TTY - input is not a terminal or the right kind of file", "Failed to attach to pod TTY; falling back to streaming logs"},
		},
		{
			name:            "stok plan",
			args:            []string{"plan", "--context", *kubectx, "--debug", "--", "-no-color", "-input=false", "-var", "suffix=foo"},
			wantExitCode:    0,
			wantStdoutRegex: regexp.MustCompile(`Refreshing Terraform state in-memory prior to plan`),
			pty:             false,
			wantWarnings:    []string{"Unable to use a TTY - input is not a terminal or the right kind of file", "Failed to attach to pod TTY; falling back to streaming logs"},
		},
		{
			name:            "stok plan with pty",
			args:            []string{"plan", "--context", *kubectx, "--", "-no-color", "-input=true"},
			wantExitCode:    0,
			wantStdoutRegex: regexp.MustCompile(`(?s)var\.suffix.*Enter a value:.*Refreshing Terraform state in-memory prior to plan`),
			pty:             true,
			stdin:           []byte("foo\n"),
		},
		{
			name:            "stok apply with pty",
			args:            []string{"apply", "--context", *kubectx, "--", "-no-color", "-input=true"},
			wantExitCode:    0,
			wantStdoutRegex: regexp.MustCompile(`Apply complete! Resources: 1 added, 0 changed, 0 destroyed.`),
			pty:             true,
			stdin:           []byte("foo\nyes\n"),
		},
		{
			name:            "stok sh",
			args:            []string{"sh", "--context", *kubectx},
			wantExitCode:    0,
			wantStdoutRegex: regexp.MustCompile(`Linux`),
			pty:             true,
			stdin:           []byte("uname; sleep 1; exit\n"),
		},
		{
			name:            "stok queuing",
			args:            []string{"sh", "--context", *kubectx, "--", "uname;", "sleep 5"},
			wantExitCode:    0,
			wantStdoutRegex: regexp.MustCompile(`Linux`),
			pty:             false,
			queueAdditional: 1,
		},
		{
			name:            "stok destroy with pty",
			args:            []string{"destroy", "--context", *kubectx, "--", "-input=true", "-var", "suffix=foo"},
			wantExitCode:    0,
			wantStdoutRegex: regexp.MustCompile(``),
			pty:             true,
			stdin:           []byte("yes\n"),
		},
		{
			name:            "delete workspace",
			args:            []string{"workspace", "delete", wsName, "--context", *kubectx},
			wantExitCode:    0,
			wantStdoutRegex: regexp.MustCompile(``),
			pty:             false,
		},
	}

	// Invoke stok with each test case
	for _, tt := range tests {
		success := t.Run(tt.name, func(t *testing.T) {
			for i := 0; i <= tt.queueAdditional; i++ {
				cmd := exec.Command(buildPath, tt.args...)
				cmd.Dir = workspacePath

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
			}
		})
		// if any one test fails then exit
		if !success {
			t.FailNow()
		}
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
			t.Fatalf("expected exit code %d, got %d\n", wantExitCode, exiterr.ExitCode())
		}
	} else if err != nil {
		t.Fatal(err)
	} else {
		// got exit code 0 and no error
		if wantExitCode != 0 {
			t.Fatalf("expected exit code %d, got 0\n", wantExitCode)
		}
	}
}
