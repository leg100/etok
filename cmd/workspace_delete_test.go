package cmd

import (
	"bytes"
	"context"
	"testing"

	v1alpha1 "github.com/leg100/stok/api/stok.goalspike.com/v1alpha1"
	"github.com/leg100/stok/pkg/k8s/stokclient"
	"github.com/leg100/stok/pkg/k8s/stokclient/fake"
	"github.com/leg100/stok/pkg/options"
	"github.com/leg100/stok/testutil"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	kfake "k8s.io/client-go/kubernetes/fake"
)

func TestDeleteWorkspace(t *testing.T) {
	ws1 := &v1alpha1.Workspace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "workspace-1",
			Namespace: "default",
		},
	}

	testutil.Run(t, "Delete", func(t *testutil.T) {
		out := new(bytes.Buffer)
		opts := &options.StokOptions{Out: out, ErrOut: out}

		cmd := root.Build(opts, testClients(ws1))

		args := []string{"workspace", "delete", "default/workspace-1"}
		assert.NoError(t, cmd.ParseAndRun(context.Background(), args))
	})
}

func testClients(objs ...runtime.Object) func(string) (stokclient.Interface, kubernetes.Interface, error) {
	var kubeObjs, stokObjs []runtime.Object
	for _, obj := range objs {
		switch obj.(type) {
		case *v1alpha1.Run, *v1alpha1.Workspace:
			stokObjs = append(stokObjs, obj)
		default:
			kubeObjs = append(kubeObjs, obj)
		}
	}

	return func(_ string) (stokclient.Interface, kubernetes.Interface, error) {
		sc := fake.NewSimpleClientset(stokObjs...)
		kc := kfake.NewSimpleClientset(kubeObjs...)
		return sc, kc, nil
	}
}
