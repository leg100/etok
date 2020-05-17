package cmd

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/fatih/structs"
	"github.com/kr/logfmt"
)

func TestDebug(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want map[string]string
	}{
		{
			name: "stok with cli flags",
			args: []string{"stok", "debug", "--namespace", "foo", "--workspace", "foo"},
			want: map[string]string{
				"Workspace":  "foo",
				"Namespace":  "foo",
				"ConfigFile": "",
			},
		},
		{
			name: "stok with config file",
			args: []string{"stok", "debug", "--config", "fixtures/config.yaml"},
			want: map[string]string{
				"Workspace":  "foo",
				"Namespace":  "foo",
				"ConfigFile": "fixtures/config.yaml",
			},
		},
		{
			name: "stok with mix",
			args: []string{"stok", "debug", "--config", "fixtures/config.yaml", "--namespace", "baz"},
			want: map[string]string{
				"Workspace":  "foo",
				"Namespace":  "baz",
				"ConfigFile": "fixtures/config.yaml",
			},
		},
	}

	// invoke stok with each test case
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// set args
			os.Args = tt.args

			old := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			debugCmd.Execute()

			w.Close()
			os.Stdout = old
			out, _ := ioutil.ReadAll(r)

			// unmarshal into struct
			lm := &LogMsg{}
			if err := logfmt.Unmarshal(out, lm); err != nil {
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
