package e2e

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"regexp"
	"testing"
	"time"

	goctx "context"

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
	namespace, err := ctx.GetNamespace()
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

	// create secret resource
	var secret = corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "secret-1",
			Namespace: "operator-test",
		},
		TypeMeta: metav1.TypeMeta{
			Kind: "Secret",
		},
		StringData: map[string]string{
			"google_application_credentials.json": creds,
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
	}{
		{
			name:            "stok",
			args:            []string{},
			wantExitCode:    127,
			wantStdoutRegex: regexp.MustCompile(`^Usage: terraform`),
		},
		{
			name:            "stok version",
			args:            []string{"version"},
			wantExitCode:    0,
			wantStdoutRegex: regexp.MustCompile(`^Terraform v0\.1`),
		},
		{
			name:            "stok init",
			args:            []string{"init", "-no-color", "-input=false"},
			wantExitCode:    0,
			wantStdoutRegex: regexp.MustCompile(`^\nInitializing the backend`),
		},
		{
			name:            "stok plan",
			args:            []string{"plan", "-no-color", "-input=false"},
			wantExitCode:    0,
			wantStdoutRegex: regexp.MustCompile(`^Refreshing Terraform state in-memory prior to plan`),
		},
	}

	// invoke stok with each test case
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command("stok", tt.args...)
			cmd.Dir = "./test/e2e/workspace"
			cmd.Env = append(os.Environ(), "STOK_NAMESPACE=operator-test", "STOK_WORKSPACE=workspace-1")

			stdout := new(bytes.Buffer)
			stderr := new(bytes.Buffer)
			cmd.Stdout = stdout
			cmd.Stderr = stderr

			err := cmd.Run()
			exitCodeTest(t, err, tt.wantExitCode)

			if !tt.wantStdoutRegex.Match(stdout.Bytes()) {
				t.Errorf("expected stdout to match %s but got %s\n", tt.wantStdoutRegex, cmd.Stdout)
			}
		})
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
