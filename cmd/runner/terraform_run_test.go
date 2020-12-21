package runner

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRunTerraformRunWithExistingWorkspace(t *testing.T) {
	out := new(bytes.Buffer)
	exec := &fakeExecutorEchoArgs{out: out}

	assert.NoError(t, execTerraformRun(context.Background(), exec, "plan", "default_foo", []string{"-out", "plan.out"}))

	want := "[terraform init -input=false -no-color -upgrade][terraform workspace select -no-color default_foo][terraform plan -out plan.out]"
	assert.Equal(t, want, strings.TrimSpace(out.String()))
}

func TestRunTerraformRunWithNewWorkspace(t *testing.T) {
	out := new(bytes.Buffer)
	exec := &fakeExecutorMissingWorkspace{out: out}

	assert.NoError(t, execTerraformRun(context.Background(), exec, "plan", "default_foo", []string{"-out", "plan.out"}))

	want := "[terraform init -input=false -no-color -upgrade][terraform workspace select -no-color default_foo][terraform workspace new -no-color default_foo][terraform plan -out plan.out]"
	assert.Equal(t, want, strings.TrimSpace(out.String()))
}
