package cmd

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/leg100/stok/pkg/apis"
	"github.com/leg100/stok/util"
	"k8s.io/kubectl/pkg/scheme"
)

func TestRunner(t *testing.T) {
	//var shell = v1alpha1.Shell{
	//	ObjectMeta: metav1.ObjectMeta{
	//		Name:      "stok-shell-xyz",
	//		Namespace: "default",
	//	},
	//	CommandSpec: v1alpha1.CommandSpec{
	//		Args: []string{"cow", "pig"},
	//	},
	//}

	path, err := ioutil.TempDir("", "test-path")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(path)

	// Create test tarball
	buf, err := util.Create("../util/fixtures/tarball", []string{"test1.tf", "test2.tf"})
	if err != nil {
		t.Fatal(err)
	}

	// Stick tarball in same dir as workspace
	tarball := filepath.Join(path, "tarball.tar.gz")
	if err := ioutil.WriteFile(tarball, buf.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}

	s := scheme.Scheme
	// adds CRD GVKs
	apis.AddToScheme(s)

	runner := &runnerCmd{
		Kind:      "Shell",
		Name:      "stok-shell-xyz",
		Namespace: "default",
		Path:      path,
		Tarball:   tarball,
		scheme:    s,

		entrypoint: "/bin/echo",
		args:       []string{"cow", "pig"},
	}

	// Fake controller-runtime client for retrieving command and configmap resources
	// 	cr := fake.NewFakeClientWithScheme(s, &shell)

	if err := runner.extractTarball(); err != nil {
		t.Fatal(err)
	}

	// Test tarball extraction
	if _, err = os.Stat(filepath.Join(path, "test1.tf", "/")); err != nil {
		t.Error(err)
	}
	if _, err = os.Stat(filepath.Join(path, "test2.tf", "/")); err != nil {
		t.Error(err)
	}

	out := new(bytes.Buffer)
	errout := new(bytes.Buffer)
	// Test running the specified program
	if err := runner.run(out, errout); err != nil {
		t.Fatal(err)
	}
	// Test console output
	want := "cow pig\n"
	got := out.String()
	if want != got {
		t.Errorf("want %#v got %#v", want, got)
	}
}
