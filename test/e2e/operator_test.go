package e2e

import (
	"bytes"
	"context"
	goctx "context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/storage"

	"github.com/kr/logfmt"
	"github.com/kr/pty"
	"golang.org/x/sys/unix"

	"github.com/leg100/stok/pkg/apis"
	terraformv1alpha1 "github.com/leg100/stok/pkg/apis/terraform/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"

	framework "github.com/operator-framework/operator-sdk/pkg/test"
	"github.com/operator-framework/operator-sdk/pkg/test/e2eutil"
)

var (
	retryInterval        = time.Second * 5
	timeout              = time.Second * 60
	cleanupRetryInterval = time.Second * 1
	cleanupTimeout       = time.Second * 5
)

func TestStok(t *testing.T) {
	workspaceList := &terraformv1alpha1.WorkspaceList{}
	err := framework.AddToFrameworkScheme(apis.AddToScheme, workspaceList)
	if err != nil {
		t.Fatalf("failed to add custom resource scheme to framework: %v", err)
	}

	commandList := &terraformv1alpha1.CommandList{}
	err = framework.AddToFrameworkScheme(apis.AddToScheme, commandList)
	if err != nil {
		t.Fatalf("failed to add custom resource scheme to framework: %v", err)
	}

	ctx := framework.NewTestCtx(t)
	defer ctx.Cleanup()

	err = ctx.InitializeClusterResources(&framework.CleanupOptions{TestContext: ctx, Timeout: cleanupTimeout, RetryInterval: cleanupRetryInterval})
	if err != nil {
		t.Fatalf("failed to initialize cluster resources: %v", err)
	}

	// get namespace
	namespace, err := ctx.GetOperatorNamespace()
	if err != nil {
		t.Fatal(err)
	}
	// get global framework variables
	f := framework.Global
	// wait for workspace-operator to be ready
	err = e2eutil.WaitForOperatorDeployment(t, f.KubeClient, namespace, "stok-operator", 1, time.Second*5, time.Second*30)
	if err != nil {
		t.Fatal(err)
	}

	// get credentials
	creds, err := getGoogleCredentials()
	if err != nil {
		t.Fatal(err)
	}

	// we want a clean backend beforehand :)
	sclient, err := storage.NewClient(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	bkt := sclient.Bucket("master-anagram-224816-tfstate")
	// ignore errors
	bkt.Object("default.tfstate").Delete(context.Background())

	// create secret resource
	var secret = corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "secret-1",
			Namespace: namespace,
		},
		TypeMeta: metav1.TypeMeta{
			Kind: "Secret",
		},
		StringData: map[string]string{
			"google-credentials.json": creds,
		},
	}
	err = f.Client.Create(goctx.TODO(), &secret, &framework.CleanupOptions{TestContext: ctx, Timeout: time.Second * 5, RetryInterval: time.Second * 1})
	if err != nil {
		t.Fatal(err)
	}

	// create workspace custom resource
	workspace := &terraformv1alpha1.Workspace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "workspace-1",
			Namespace: namespace,
		},
		TypeMeta: metav1.TypeMeta{
			Kind: "Workspace",
		},
		Spec: terraformv1alpha1.WorkspaceSpec{
			SecretName: "secret-1",
		},
	}
	err = f.Client.Create(goctx.TODO(), workspace, &framework.CleanupOptions{TestContext: ctx, Timeout: time.Second * 5, RetryInterval: time.Second * 1})
	if err != nil {
		t.Fatal(err)
	}

	err = wait.Poll(retryInterval, timeout, func() (done bool, err error) {
		err = f.Client.Get(goctx.TODO(), types.NamespacedName{Namespace: namespace, Name: "workspace-1"}, &corev1.PersistentVolumeClaim{})

		if err != nil {
			return false, err
		}

		return true, nil
	})
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name            string
		args            []string
		path            string
		wantExitCode    int
		wantStdoutRegex *regexp.Regexp
		pty             bool
		wantWarnings    []string
		stdin           []byte
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
			args:            []string{"version"},
			wantExitCode:    0,
			wantStdoutRegex: regexp.MustCompile(`^Terraform v0\.1`),
			pty:             false,
			wantWarnings:    []string{"Unable to use a TTY - input is not a terminal or the right kind of file", "Failed to attach to pod TTY; falling back to streaming logs"},
		},
		{
			name:            "stok init",
			args:            []string{"init", "--", "-no-color", "-input=false"},
			wantExitCode:    0,
			wantStdoutRegex: regexp.MustCompile(`^\nInitializing the backend`),
			pty:             false,
			wantWarnings:    []string{"Unable to use a TTY - input is not a terminal or the right kind of file", "Failed to attach to pod TTY; falling back to streaming logs"},
		},
		{
			name:            "stok plan",
			args:            []string{"plan", "--", "-no-color", "-input=false", "-var 'suffix=foo'"},
			wantExitCode:    0,
			wantStdoutRegex: regexp.MustCompile(`^Refreshing Terraform state in-memory prior to plan`),
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
	}

	// invoke stok with each test case
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command("../../../build/_output/bin/stok", tt.args...)
			cmd.Dir = "./test/e2e/workspace"
			cmd.Env = append(os.Environ(), fmt.Sprintf("STOK_NAMESPACE=%s", namespace), "STOK_WORKSPACE=workspace-1")

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

			err = cmd.Wait()
			exitCodeTest(t, err, tt.wantExitCode)

			// Without a pty we expect a warning log msg telling us as much.
			// (We can use stderr without pty but not with pty)
			if !tt.pty {
				for idx, warning := range tt.wantWarnings {
					data := strings.Split(errbuf.String(), "\n")[idx]

					got := &LogMsg{}
					if err = logfmt.Unmarshal([]byte(data), got); err != nil {
						t.Fatal(err)
					}
					if got.Level != "warning" {
						t.Errorf("want level=warning, got level=%s\n", got.Level)
					}
					if got.Msg != warning {
						t.Errorf("want message='%s', got message=%s\n", warning, got.Msg)
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

type LogMsg struct {
	Time  string
	Level string
	Msg   string
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

func getGoogleCredentials() (string, error) {
	path := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
	if path == "" {
		return "", fmt.Errorf("Could not find env var GOOGLE_APPLICATION_CREDENTIALS")
	}

	bytes, err := ioutil.ReadFile(path)
	if os.IsNotExist(err) {
		return "", fmt.Errorf("Env var GOOGLE_APPLICATION_CREDENTIALS resolves to %s but %s does not exist\n", path, path)
	}
	if err != nil {
		return "", err
	}

	return string(bytes), nil
}
