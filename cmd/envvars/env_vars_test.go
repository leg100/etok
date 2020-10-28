package envvars

import (
	"testing"

	"github.com/leg100/stok/testutil"
	"github.com/spf13/cobra"
	"gotest.tools/assert"
)

func TestSetFlagsFromEnvVariables(t *testing.T) {
	tests := []struct {
		name    string
		envs    map[string]string
		want    string
		setFlag func(*cobra.Command) *string
	}{
		{
			name: "non-persistent flag",
			envs: map[string]string{
				"STOK_FOO": "override",
			},
			want: "override",
			setFlag: func(cmd *cobra.Command) *string {
				return cmd.Flags().String("foo", "default", "")
			},
		},
		{
			name: "persistent flag",
			envs: map[string]string{
				"STOK_FOO": "override",
			},
			want: "override",
			setFlag: func(cmd *cobra.Command) *string {
				return cmd.PersistentFlags().String("foo", "default", "")
			},
		},
		{
			name: "flag on child command",
			envs: map[string]string{
				"STOK_FOO": "override",
			},
			want: "override",
			setFlag: func(parent *cobra.Command) *string {
				child := &cobra.Command{}
				foo := child.Flags().String("foo", "default", "")
				parent.AddCommand(child)
				return foo
			},
		},
		{
			name: "persistent flag on child command",
			envs: map[string]string{
				"STOK_FOO": "override",
			},
			want: "override",
			setFlag: func(parent *cobra.Command) *string {
				child := &cobra.Command{}
				foo := child.PersistentFlags().String("foo", "default", "")
				parent.AddCommand(child)
				return foo
			},
		},
		{
			name: "flag on grandchild command",
			envs: map[string]string{
				"STOK_FOO": "override",
			},
			want: "override",
			setFlag: func(grandparent *cobra.Command) *string {
				parent := &cobra.Command{}
				grandchild := &cobra.Command{}
				foo := grandchild.Flags().String("foo", "default", "")
				grandparent.AddCommand(parent)
				parent.AddCommand(grandchild)
				return foo
			},
		},
		{
			name: "misnamed",
			envs: map[string]string{
				"STOK_FOOBAR": "override",
			},
			want: "default",
			setFlag: func(cmd *cobra.Command) *string {
				foo := cmd.Flags().String("foo", "default", "")
				return foo
			},
		},
	}
	for _, tt := range tests {
		testutil.Run(t, tt.name, func(t *testutil.T) {
			t.SetEnvs(tt.envs)
			cmd := &cobra.Command{}
			foo := tt.setFlag(cmd)

			SetFlagsFromEnvVariables(cmd)

			assert.Equal(t, *foo, tt.want)
		})
	}
}
