package flags

import (
	"flag"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStringToString(t *testing.T) {
	tests := []struct {
		name  string
		def   map[string]string
		flags []string
		want  map[string]string
	}{
		{
			name: "empty default",
			def:  map[string]string{},
			want: map[string]string{},
		},
		{
			name: "default",
			def:  map[string]string{"defk1": "defv1"},
			want: map[string]string{"defk1": "defv1"},
		},
		{
			name:  "override default",
			def:   map[string]string{"defk1": "defv1"},
			flags: []string{"-config", "k1=v1,k2=v2"},
			want:  map[string]string{"k1": "v1", "k2": "v2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := flag.NewFlagSet("test", flag.ContinueOnError)
			got := StringToStringFlag(fs, "config", tt.def, "usage msg")

			require.NoError(t, fs.Parse(tt.flags))

			assert.Equal(t, *got, tt.want)
		})
	}
}
