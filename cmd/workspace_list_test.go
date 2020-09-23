package cmd

import (
	"bytes"
	"testing"

	v1alpha1types "github.com/leg100/stok/api/stok.goalspike.com/v1alpha1"
	"github.com/leg100/stok/pkg/k8s"
	"github.com/leg100/stok/pkg/k8s/stokclient"
	"github.com/leg100/stok/pkg/k8s/stokclient/fake"
	"github.com/leg100/stok/testutil"
	"github.com/leg100/stok/util"
	"github.com/stretchr/testify/require"
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

	testutil.Run(t, "WithEnvironmentFile", func(t *testutil.T) {
		path := t.NewTempDir().Root()
		err := util.WriteEnvironmentFile(path, "default", "workspace-1")
		require.NoError(t, err)

		fakeStokClient := fake.NewSimpleClientset(ws1, ws2)
		t.Override(&k8s.StokClient, func() (stokclient.Interface, error) {
			return fakeStokClient, nil
		})

		out := new(bytes.Buffer)
		cmd := newStokCmd(out, out)

		code, err := cmd.Execute([]string{
			"workspace",
			"list",
			"--path", path,
		})
		require.NoError(t, err)
		require.Equal(t, 0, code)
		require.Equal(t, "*\tdefault/workspace-1\n\tdev/workspace-2\n", out.String())
	})

	testutil.Run(t, "WithoutEnvironmentFile", func(t *testutil.T) {
		path := t.NewTempDir().Root()

		fakeStokClient := fake.NewSimpleClientset(ws1, ws2)
		t.Override(&k8s.StokClient, func() (stokclient.Interface, error) {
			return fakeStokClient, nil
		})

		out := new(bytes.Buffer)
		cmd := newStokCmd(out, out)

		code, err := cmd.Execute([]string{
			"workspace",
			"list",
			"--path", path,
		})
		require.NoError(t, err)
		require.Equal(t, 0, code)
		require.Equal(t, "\tdefault/workspace-1\n\tdev/workspace-2\n", out.String())
	})
}
