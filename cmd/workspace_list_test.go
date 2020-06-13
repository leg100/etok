package cmd

import (
	"bytes"
	"testing"

	v1alpha1types "github.com/leg100/stok/pkg/apis/stok/v1alpha1"
	fakeStokClient "github.com/leg100/stok/pkg/client/clientset/fake"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestListWorkspaces(t *testing.T) {
	ws1 := &v1alpha1types.Workspace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "workspace-1",
			Namespace: "default",
		},
	}

	ws2 := &v1alpha1types.Workspace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "workspace-2",
			Namespace: "dev",
		},
	}

	clientset := fakeStokClient.NewSimpleClientset(ws1, ws2)

	lwc := &listWorkspaceCmd{}

	out := new(bytes.Buffer)

	if err := lwc.listWorkspaces(clientset.StokV1alpha1(), "workspace-1", out); err != nil {
		t.Fatal(err)
	}

	want := "*\tworkspace-1\n\tworkspace-2\n"
	got := out.String()
	if want != got {
		t.Errorf("want %s got %s", want, got)
	}
}
