package cmd

import (
	"context"
	"testing"

	"github.com/leg100/stok/pkg/apis"
	"github.com/leg100/stok/pkg/apis/stok/v1alpha1"
	v1alpha1types "github.com/leg100/stok/pkg/apis/stok/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubectl/pkg/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestDeleteWorkspace(t *testing.T) {
	ws1 := &v1alpha1types.Workspace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "workspace-1",
			Namespace: "default",
		},
	}

	s := scheme.Scheme
	// adds CRD GVKs
	apis.AddToScheme(s)

	client := fake.NewFakeClientWithScheme(s, ws1)

	lwc := &deleteWorkspaceCmd{Namespace: "default", Name: "workspace-1"}

	if err := lwc.deleteWorkspace(client); err != nil {
		t.Fatal(err)
	}

	workspaces := v1alpha1.WorkspaceList{}
	// List across all namespaces
	if err := client.List(context.TODO(), &workspaces); err != nil {
		t.Fatal(err)
	}

	want := 0
	got := len(workspaces.Items)
	if want != got {
		t.Errorf("want %d got %d", want, got)
	}
}
