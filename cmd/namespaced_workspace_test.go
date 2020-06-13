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

	nw := newNamespacedWorkspace("default", "test-env")
	if err := nw.writeEnvironmentFile(dest); err != nil {
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

func TestValidateNamespacedWorkspace(t *testing.T) {
	t.Run("Valid", func(t *testing.T) {
		if err := namespacedWorkspace("a/b").validate(); err != nil {
			t.Errorf("want no error, got %v", err)
		}
	})

	t.Run("Invalid arg with multiple forward slashes", func(t *testing.T) {
		if err := namespacedWorkspace("a/b/c").validate(); err == nil {
			t.Errorf("want error, got nil")
		}
	})

	t.Run("Invalid arg with underscores", func(t *testing.T) {
		if err := namespacedWorkspace("a_b/c").validate(); err == nil {
			t.Errorf("want error, got nil")
		}
	})
}
