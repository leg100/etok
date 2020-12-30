package runner

import (
	"testing"

	"github.com/leg100/etok/pkg/testutil"
	"github.com/stretchr/testify/assert"
)

func TestPrepareArgs(t *testing.T) {
	tests := []struct {
		name    string
		command string
		args    []string
		want    []string
	}{
		{
			name:    "shell with no args",
			command: "sh",
			want:    []string{"sh"},
		},
		{
			name:    "shell with args",
			command: "sh",
			args:    []string{"echo", "foo"},
			want:    []string{"sh", "-c", "echo foo"},
		},
		{
			name:    "terraform plan",
			command: "plan",
			want:    []string{"terraform", "plan"},
		},
		{
			name:    "terraform plan with args",
			command: "plan",
			args:    []string{"-input", "false"},
			want:    []string{"terraform", "plan", "-input", "false"},
		},
		{
			name:    "terraform state pull",
			command: "state pull",
			want:    []string{"terraform", "state", "pull"},
		},
		{
			name:    "terraform state pull with args",
			command: "state pull",
			args:    []string{"-input", "false"},
			want:    []string{"terraform", "state", "pull", "-input", "false"},
		},
	}

	for _, tt := range tests {
		testutil.Run(t, tt.name, func(t *testutil.T) {
			assert.Equal(t, tt.want, prepareArgs(tt.command, tt.args...))
		})
	}
}
