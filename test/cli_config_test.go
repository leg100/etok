package e2e

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"testing"

	"github.com/fatih/structs"
	"github.com/kr/logfmt"
)

func TestStok(t *testing.T) {
	tests := []struct {
		name string
		args []string
		env  []string
		want map[string]string
	}{
		{
			name: "stok with env vars",
			args: []string{"debug"},
			env:  []string{"STOK_NAMESPACE=foo", "STOK_WORKSPACE=foo"},
			want: map[string]string{
				"Workspace":  "foo",
				"Namespace":  "foo",
				"ConfigFile": "",
			},
		},
		{
			name: "stok with cli flags",
			args: []string{"debug", "--namespace", "foo", "--workspace", "foo"},
			env:  []string{},
			want: map[string]string{
				"Workspace":  "foo",
				"Namespace":  "foo",
				"ConfigFile": "",
			},
		},
		{
			name: "stok with config file",
			args: []string{"debug", "--config", "fixtures/config.yaml"},
			env:  []string{},
			want: map[string]string{
				"Workspace":  "foo",
				"Namespace":  "foo",
				"ConfigFile": "fixtures/config.yaml",
			},
		},
		{
			name: "stok with mix",
			args: []string{"debug", "--config", "fixtures/config.yaml", "--namespace", "baz"},
			env:  []string{"STOK_WORKSPACE=bar"},
			want: map[string]string{
				"Workspace":  "bar",
				"Namespace":  "baz",
				"ConfigFile": "fixtures/config.yaml",
			},
		},
	}

	// invoke stok with each test case
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command("../build/_output/bin/stok", tt.args...)
			cmd.Env = append([]string{"PATH=/usr/bin:/bin"}, tt.env...)

			// print to stdout too to aid with debugging
			buf := new(bytes.Buffer)
			cmd.Stdout = io.MultiWriter(buf, os.Stdout)

			if err := cmd.Run(); err != nil {
				t.Fatal(err)
			}

			// unmarshal into struct
			lm := &LogMsg{}
			if err := logfmt.Unmarshal(buf.Bytes(), lm); err != nil {
				t.Fatal(err)
			}
			// convert into map
			got := structs.Map(lm)

			// compare each k,v in got map with want map
			for k, wantV := range tt.want {
				gotV, ok := got[k]
				if !ok {
					t.Fatalf("Could not find key '%s' in map '%+v'\n", k, got)
				}
				if wantV != gotV {
					t.Errorf("want %s=%s, got %s=%s\n", k, wantV, k, gotV)
				}
			}

		})
	}
}

type LogMsg struct {
	Workspace  string
	Namespace  string
	ConfigFile string
}
