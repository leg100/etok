package cmd

import (
	"testing"

	v1alpha1types "github.com/leg100/stok/pkg/apis/stok/v1alpha1"
	fakeStokClient "github.com/leg100/stok/pkg/client/clientset/fake"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestDeleteWorkspace(t *testing.T) {
	ws1 := &v1alpha1types.Workspace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "workspace-1",
			Namespace: "default",
		},
	}

	clientset := fakeStokClient.NewSimpleClientset(ws1)

	lwc := &deleteWorkspaceCmd{Namespace: "default", Name: "workspace-1"}

	if err := lwc.deleteWorkspace(clientset.StokV1alpha1()); err != nil {
		t.Fatal(err)
	}

	workspaces, err := clientset.StokV1alpha1().Workspaces("default").List(metav1.ListOptions{})
	if err != nil {
		t.Fatal(err)
	}

	want := 0
	got := len(workspaces.Items)
	if want != got {
		t.Errorf("want %d got %d", want, got)
	}
}
