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
	"time"

	"cloud.google.com/go/storage"

	expect "github.com/google/goexpect"
	"github.com/stretchr/testify/require"
)

const (
	buildPath     = "../../stok"
	workspacePath = "./workspace"
	backendBucket = "automatize-tfstate"
	backendPrefix = "e2e"

	// Namespace in which stok workspace will be created in,
	// and commands tested in
	wsNamespace = "default"

	// Name of workspace to be created
	wsName = "default/foo"

	// Name of second workspace to be created
	wsName2 = "default/bar"
)

var kubectx = flag.String("context", "kind-kind", "Kubeconfig context to use for tests")

// End-to-end tests
func TestStok(t *testing.T) {
	fmt.Printf("Kubernetes context set to: %s\n", *kubectx)

	// we want a clean backend beforehand
	sclient, err := storage.NewClient(goctx.Background())
	require.NoError(t, err)
	bkt := sclient.Bucket(backendBucket)
	// ignore errors
	bkt.Object(backendPrefix + "/default-foo.tfstate").Delete(goctx.Background())
	bkt.Object(backendPrefix + "/default-foo.tflock").Delete(goctx.Background())

	tests := []struct {
		name            string
		args            []string
		path            string
		wantExitCode    int
		wantStdoutRegex *regexp.Regexp
		pty             bool
		stdin           []byte
		batch           []expect.Batcher
		queueAdditional int
	}{
		{
			name:            "new workspace",
			args:            []string{"workspace", "new", wsName, "--path", "workspace", "--timeout-pod", "60s", "--context", *kubectx, "--backend-type", "gcs", "--backend-config", "bucket=automatize-tfstate,prefix=e2e", "--privileged-commands", "apply"},
			wantExitCode:    0,
			wantStdoutRegex: regexp.MustCompile(``),
			pty:             false,
		},
		{
			name:            "second new workspace",
			args:            []string{"workspace", "new", wsName2, "--path", "workspace", "--timeout-pod", "60s", "--context", *kubectx, "--terraform-version", "0.12.17"},
			wantExitCode:    0,
			wantStdoutRegex: regexp.MustCompile(``),
			pty:             false,
		},
		{
			name:            "list workspaces",
			args:            []string{"workspace", "list", "--path", "workspace", "--context", *kubectx},
			wantExitCode:    0,
			wantStdoutRegex: regexp.MustCompile(fmt.Sprintf("\\*\t%s\n\t%s", wsName2, wsName)),
			pty:             false,
		},
		{
			name:            "select first workspace",
			args:            []string{"workspace", "select", "--path", "workspace", wsName},
			wantExitCode:    0,
			wantStdoutRegex: regexp.MustCompile(``),
			pty:             false,
		},
		{
			name:            "show current workspace",
			args:            []string{"workspace", "show", "--path", "workspace"},
			wantExitCode:    0,
			wantStdoutRegex: regexp.MustCompile(wsName),
			pty:             false,
		},
		{
			name:            "stok init without pty",
			args:            []string{"init", "--path", "workspace", "--context", *kubectx, "--", "-no-color", "-input=false"},
			wantExitCode:    0,
			wantStdoutRegex: regexp.MustCompile(`Initializing the backend`),
			pty:             false,
		},
		{
			name:            "stok plan without pty",
			args:            []string{"plan", "--path", "workspace", "--context", *kubectx, "--", "-no-color", "-input=false", "-var", "suffix=foo"},
			wantExitCode:    0,
			wantStdoutRegex: regexp.MustCompile(`Refreshing Terraform state in-memory prior to plan`),
			pty:             false,
		},
		{
			name:         "stok plan with pty",
			args:         []string{"plan", "--path", "workspace", "--context", *kubectx, "--", "-input=true", "-no-color"},
			wantExitCode: 0,
			pty:          true,
			batch: []expect.Batcher{
				&expect.BExp{R: `Enter a value:`},
				&expect.BSnd{S: "foo\n"},
				&expect.BExp{R: `Plan: 1 to add, 0 to change, 0 to destroy.`},
			},
		},
		{
			name:         "stok apply with pty",
			args:         []string{"apply", "--path", "workspace", "--context", *kubectx, "--", "-input=true", "-no-color"},
			wantExitCode: 0,
			pty:          true,
			batch: []expect.Batcher{
				&expect.BExp{R: `Enter a value:`},
				&expect.BSnd{S: "foo\n"},
				&expect.BExp{R: `Enter a value:`},
				&expect.BSnd{S: "yes\n"},
				&expect.BExp{R: `Apply complete! Resources: 1 added, 0 changed, 0 destroyed.`},
			},
		},
		{
			name:         "stok sh",
			args:         []string{"sh", "--path", "workspace", "--context", *kubectx},
			wantExitCode: 0,
			pty:          true,
			batch: []expect.Batcher{
				&expect.BExp{R: `#`},
				&expect.BSnd{S: "uname; exit\n"},
				&expect.BExp{R: `Linux`},
			},
		},
		{
			name:            "stok queuing",
			args:            []string{"sh", "--path", "workspace", "--context", *kubectx, "--", "uname;", "sleep 5"},
			wantExitCode:    0,
			wantStdoutRegex: regexp.MustCompile(`Linux`),
			pty:             false,
			queueAdditional: 1,
		},
		{
			name:         "stok destroy with pty",
			args:         []string{"destroy", "--path", "workspace", "--context", *kubectx, "--", "-input=true", "-var", "suffix=foo", "-no-color"},
			wantExitCode: 0,
			pty:          true,
			batch: []expect.Batcher{
				&expect.BExp{R: `Enter a value:`},
				&expect.BSnd{S: "yes\n"},
				&expect.BExp{R: `Destroy complete! Resources: 1 destroyed.`},
			},
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
				args := append([]string{buildPath}, tt.args...)
				if tt.pty {
					exp, _, err := expect.SpawnWithArgs(args, 10*time.Second, expect.Tee(nopWriteCloser{os.Stdout}))
					require.NoError(t, err)
					defer exp.Close()

					_, err = exp.ExpectBatch(tt.batch, 10*time.Second)
					require.NoError(t, err)
				} else {
					cmd := exec.Command(args[0], args[1:]...)

					outbuf := new(bytes.Buffer)
					out := io.MultiWriter(outbuf, os.Stdout)

					// without pty, so just use a buffer, and skip stdin
					cmd.Stdout = out
					cmd.Stderr = os.Stderr

					require.NoError(t, cmd.Start())

					exitCodeTest(t, cmd.Wait(), tt.wantExitCode)

					if tt.wantStdoutRegex != nil {
						require.Regexp(t, tt.wantStdoutRegex, outbuf.String())
					}
				}
			}
		})
		require.True(t, success)
	}
}

type nopWriteCloser struct {
	f *os.File
}

func (n nopWriteCloser) Write(p []byte) (int, error) {
	return n.f.Write(p)
}

func (n nopWriteCloser) Close() error {
	return nil
}

func exitCodeTest(t *testing.T, err error, wantExitCode int) {
	if exiterr, ok := err.(*exec.ExitError); ok {
		require.Equal(t, wantExitCode, exiterr.ExitCode())
	} else if err != nil {
		require.NoError(t, err)
	} else {
		// got exit code 0; ensures thats whats wanted
		require.Equal(t, wantExitCode, 0)
	}
}
