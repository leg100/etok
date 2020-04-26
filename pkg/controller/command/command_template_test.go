package command

import (
	"testing"

	"github.com/leg100/stok/pkg/apis/terraform/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestScriptPlan(t *testing.T) {
	command := &v1alpha1.Command{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cmd-xxx",
			Namespace: "test-ns",
		},
		Spec: v1alpha1.CommandSpec{
			Command: []string{"terraform"},
			Args:    []string{"plan"},
		},
	}

	got, err := generateScript(command)
	if err != nil {
		t.Fatal(err)
	}

	want := `# wait for client to inform us they're streaming logs
/kubectl/kubectl wait --for=condition=ClientReady command/cmd-xxx > /dev/null

# run stok command
terraform plan

`

	if want != got {
		t.Errorf("want '%s', got '%s'\n", want, got)
	}
}

func TestScriptApply(t *testing.T) {
	command := &v1alpha1.Command{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cmd-xxx",
			Namespace: "test-ns",
		},
		Spec: v1alpha1.CommandSpec{
			Command: []string{"terraform"},
			Args:    []string{"apply"},
		},
	}

	got, err := generateScript(command)
	if err != nil {
		t.Fatal(err)
	}

	want := `# wait for client to inform us they're streaming logs
/kubectl/kubectl wait --for=condition=ClientReady command/cmd-xxx > /dev/null

# run stok command
terraform apply

# capture outputs if apply command was run
terraform output -json \
	| jq -r 'to_entries
		| map(select(.value.sensitive | not))
		| map("\(.key)=\(.value.value)")
		| .[]' \
	> outputs.env

# persist outputs to configmap
kubectl create configmap cmd-xxx-state --from-env-file=outputs.env
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
			Command: []string{"sh"},
			Args:    []string{},
		},
	}

	got, err := generateScript(command)
	if err != nil {
		t.Fatal(err)
	}

	want := `# wait for client to inform us they're streaming logs
/kubectl/kubectl wait --for=condition=ClientReady command/cmd-xxx > /dev/null

# run stok command
sh

`

	if want != got {
		t.Errorf("want '%s', got '%s'\n", want, got)
	}
}
