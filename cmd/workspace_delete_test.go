package cmd

import (
	"bytes"
	"testing"

	v1alpha1 "github.com/leg100/stok/api/stok.goalspike.com/v1alpha1"
	"github.com/leg100/stok/pkg/k8s"
	"github.com/leg100/stok/pkg/k8s/stokclient"
	"github.com/leg100/stok/pkg/k8s/stokclient/fake"
	"github.com/leg100/stok/testutil"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestDeleteWorkspace(t *testing.T) {
	ws1 := &v1alpha1.Workspace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "workspace-1",
			Namespace: "default",
		},
	}

	testutil.Run(t, "Delete", func(t *testutil.T) {
		fakeStokClient := fake.NewSimpleClientset(ws1)
		t.Override(&k8s.StokClient, func() (stokclient.Interface, error) {
			return fakeStokClient, nil
		})

		out := new(bytes.Buffer)
		code, err := newStokCmd(out, out).Execute([]string{
			"workspace",
			"delete",
			"workspace-1",
		})

		require.NoError(t, err)
		require.Equal(t, 0, code)
	})
}
