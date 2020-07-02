package cmd

import (
	"io/ioutil"
	"os"
	"path"
	"testing"
)

func TestWriteEnvironmentFile(t *testing.T) {
	dest, err := ioutil.TempDir("", "workspace-new-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dest)

	if err := writeEnvironmentFile(dest, "default", "test-env"); err != nil {
		t.Fatal(err)
	}

	wantEnvFile := path.Join(dest, ".terraform", "environment")
	gotEnv, err := ioutil.ReadFile(wantEnvFile)
	if err != nil {
		t.Fatal(err)
	}

	want := "default/test-env"
	if string(gotEnv) != want {
		t.Errorf("want %s got %s", want, string(gotEnv))
	}
}
