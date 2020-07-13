package cmd

import (
	"bytes"
	"testing"

	"github.com/leg100/stok/pkg/apis"
	v1alpha1types "github.com/leg100/stok/pkg/apis/stok/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubectl/pkg/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
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

	s := scheme.Scheme
	// adds CRD GVKs
	apis.AddToScheme(s)

	client := fake.NewFakeClientWithScheme(s, ws1, ws2)

	lwc := &listWorkspaceCmd{}

	out := new(bytes.Buffer)

	if err := lwc.listWorkspaces(client, "default", "workspace-1", out); err != nil {
		t.Fatal(err)
	}

	want := "*\tdefault/workspace-1\n\tdev/workspace-2\n"
	got := out.String()
	if want != got {
		t.Errorf("want %s got %s", want, got)
	}
}
