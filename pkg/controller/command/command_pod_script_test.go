package command

import (
	"testing"

	"github.com/leg100/stok/constants"
)

func TestScriptPlan(t *testing.T) {
	got, err := Script{
		CommandName: "cmd-xxx",
		Tarball:     constants.Tarball,
		Command:     []string{"terraform"},
		Args:        []string{"plan"},
	}.generate()
	if err != nil {
		t.Fatal(err)
	}

	want := `#Extract workspace tarball
tar zxf /tarball/tarball.tar.gz

# wait for both the client to be ready and
# for the command to be front of the workspace queue
kubectl wait --for=condition=WorkspaceReady --timeout=-1s command/cmd-xxx > /dev/null
kubectl wait --for=condition=ClientReady --timeout=-1s command/cmd-xxx > /dev/null

# run stok command
terraform plan

`

	if want != got {
		t.Errorf("want '%s', got '%s'\n", want, got)
	}
}

func TestScriptShell(t *testing.T) {
	got, err := Script{
		CommandName: "cmd-xxx",
		Tarball:     constants.Tarball,
		Command:     []string{"sh"},
		Args:        []string{},
	}.generate()
	if err != nil {
		t.Fatal(err)
	}

	want := `#Extract workspace tarball
tar zxf /tarball/tarball.tar.gz

# wait for both the client to be ready and
# for the command to be front of the workspace queue
kubectl wait --for=condition=WorkspaceReady --timeout=-1s command/cmd-xxx > /dev/null
kubectl wait --for=condition=ClientReady --timeout=-1s command/cmd-xxx > /dev/null

# run stok command
sh

`

	if want != got {
		t.Errorf("want '%s', got '%s'\n", want, got)
	}
}
