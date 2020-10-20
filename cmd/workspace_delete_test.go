package cmd

import (
	"bytes"
	"context"
	"testing"

	v1alpha1 "github.com/leg100/stok/api/stok.goalspike.com/v1alpha1"
	"github.com/leg100/stok/pkg/app"
	"github.com/leg100/stok/testutil"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestDeleteWorkspace(t *testing.T) {
	ws1 := &v1alpha1.Workspace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "workspace-1",
			Namespace: "default",
		},
	}

	tests := []struct {
		name string
		args []string
		objs []runtime.Object
		err  bool
	}{
		{
			name: "With workspace",
			args: []string{"workspace", "delete", "workspace-1"},
			objs: []runtime.Object{ws1},
		},
		{
			name: "Without workspace",
			args: []string{"workspace", "delete", "workspace-1"},
			err:  true,
		},
	}
	for _, tt := range tests {
		testutil.Run(t, tt.name, func(t *testutil.T) {
			opts, err := app.NewFakeOpts(new(bytes.Buffer), tt.objs...)
			require.NoError(t, err)

			t.CheckError(tt.err, ParseArgs(context.Background(), tt.args, opts))
		})
	}
}
