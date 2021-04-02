package github

import (
	"testing"

	"github.com/leg100/etok/pkg/testutil"
	"github.com/stretchr/testify/assert"
)

func TestRunScript(t *testing.T) {
	tests := []struct {
		name     string
		id       string
		command  string
		previous string
		want     string
	}{
		{
			name:     "default",
			id:       "run-12345",
			command:  "plan",
			previous: "",
			want:     "set -e\n\nterraform init -no-color -input=false\n\n\nterraform plan -no-color -input=false -out=/plans/run-12345\n\n",
		},
		{
			name:     "default",
			id:       "run-12345",
			command:  "apply",
			previous: "run-12345",
			want:     "set -e\n\nterraform init -no-color -input=false\n\n\n\nterraform apply -no-color -input=false /plans/run-12345\n",
		},
	}

	for _, tt := range tests {
		testutil.Run(t, tt.name, func(t *testutil.T) {
			assert.Equal(t, tt.want, runScript(tt.id, tt.command, tt.previous))
		})
	}
}
