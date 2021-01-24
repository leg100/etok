package cmd

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	cmdutil "github.com/leg100/etok/cmd/util"
	"github.com/leg100/etok/pkg/testutil"
	"github.com/leg100/etok/pkg/version"
	"github.com/stretchr/testify/assert"
)

func TestVersion(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		objs       []runtime.Object
		err        error
		assertions func(t *testutil.T, out ...string)
	}{
		{
			name: "with server install",
			objs: []runtime.Object{
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "etok",
						Namespace: "etok",
						Labels: map[string]string{
							"version": "124",
							"commit":  "abc",
						},
					},
				},
			},
			assertions: func(t *testutil.T, out ...string) {
				assert.Equal(t, "Client Version: 123\txyz", out[0])
				assert.Equal(t, "Server Version: 124\tabc", out[1])
			},
		},
		{
			name: "without server install",
			assertions: func(t *testutil.T, out ...string) {
				assert.Equal(t, "Client Version: 123\txyz", out[0])
				assert.Equal(t, "Server Version: deployment etok/etok not found", out[1])
			},
		},
	}

	for _, tt := range tests {
		testutil.Run(t, tt.name, func(t *testutil.T) {
			// Set client version and commit
			t.Override(&version.Version, "123")
			t.Override(&version.Commit, "xyz")

			out := new(bytes.Buffer)
			f := cmdutil.NewFakeFactory(out, tt.objs...)

			cmd := versionCmd(f)
			cmd.SetOut(out)
			cmd.SetArgs(tt.args)

			err := cmd.ExecuteContext(context.Background())
			if !assert.True(t, errors.Is(err, tt.err)) {
				t.Logf("wanted %v but got %v", tt.err, err)
			}

			if tt.assertions != nil {
				tt.assertions(t, strings.Split(out.String(), "\n")...)
			}
		})
	}
}
