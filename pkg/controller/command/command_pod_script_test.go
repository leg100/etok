package command

import (
	"testing"

	"github.com/leg100/stok/constants"
)

func TestScriptPlan(t *testing.T) {
	got, err := Script{
		Resource:   "stok-plan-xxx",
		Tarball:    constants.Tarball,
		Kind:       "plan",
		Entrypoint: []string{"terraform", "plan"},
	}.generate()
	if err != nil {
		t.Fatal(err)
	}

	want := `#Extract workspace tarball
tar zxf /tarball/tarball.tar.gz

# wait for both the client to be ready and
# for the command to be front of the workspace queue
kubectl wait --for=condition=WorkspaceReady --timeout=-1s plan/stok-plan-xxx > /dev/null
kubectl wait --for=condition=ClientReady --timeout=-1s plan/stok-plan-xxx > /dev/null

# run stok command
terraform plan

`

	if want != got {
		t.Errorf("want '%s', got '%s'\n", want, got)
	}
}

func TestScriptShell(t *testing.T) {
	got, err := Script{
		Resource:   "stok-shell-xxx",
		Tarball:    constants.Tarball,
		Kind:       "shell",
		Entrypoint: []string{"sh"},
		Args:       []string{"-c", "\"foo bar\""},
	}.generate()
	if err != nil {
		t.Fatal(err)
	}

	want := `#Extract workspace tarball
tar zxf /tarball/tarball.tar.gz

# wait for both the client to be ready and
# for the command to be front of the workspace queue
kubectl wait --for=condition=WorkspaceReady --timeout=-1s shell/stok-shell-xxx > /dev/null
kubectl wait --for=condition=ClientReady --timeout=-1s shell/stok-shell-xxx > /dev/null

# run stok command
sh -c "foo bar"

`

	if want != got {
		t.Errorf("want '%s', got '%s'\n", want, got)
	}
}
