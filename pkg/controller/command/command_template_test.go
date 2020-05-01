package command

import (
	"testing"

	"github.com/leg100/stok/pkg/apis/stok/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestScriptPlan(t *testing.T) {
	command := &v1alpha1.Command{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cmd-xxx",
			Namespace: "test-ns",
		},
		Spec: v1alpha1.CommandSpec{
			Command:      []string{"terraform"},
			Args:         []string{"plan"},
			ConfigMapKey: "tarball.tar.gz",
		},
	}

	got, err := generateScript(command)
	if err != nil {
		t.Fatal(err)
	}

	want := `#Extract workspace tarball
tar zxf /tarball/tarball.tar.gz

# wait for both the client to be ready and
# for the command to be front of the workspace queue
kubectl wait --for=condition=ClientReady command/cmd-xxx > /dev/null
kubectl wait --for=condition=FrontOfQueue command/cmd-xxx > /dev/null

# run stok command
terraform plan

`

	if want != got {
		t.Errorf("want '%s', got '%s'\n", want, got)
	}
}

func TestScriptShell(t *testing.T) {
	command := &v1alpha1.Command{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cmd-xxx",
			Namespace: "test-ns",
		},
		Spec: v1alpha1.CommandSpec{
			Command:      []string{"sh"},
			Args:         []string{},
			ConfigMapKey: "tarball.tar.gz",
		},
	}

	got, err := generateScript(command)
	if err != nil {
		t.Fatal(err)
	}

	want := `#Extract workspace tarball
tar zxf /tarball/tarball.tar.gz

# wait for both the client to be ready and
# for the command to be front of the workspace queue
kubectl wait --for=condition=ClientReady command/cmd-xxx > /dev/null
kubectl wait --for=condition=FrontOfQueue command/cmd-xxx > /dev/null

# run stok command
sh

`

	if want != got {
		t.Errorf("want '%s', got '%s'\n", want, got)
	}
}
