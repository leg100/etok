package util

import (
	"encoding/base64"
	"io/ioutil"
	"os"
	"testing"
)

func TestCreate(t *testing.T) {
	buf, err := Create("fixtures/tarball", []string{"test1.tf", "test2.tf"})
	if err != nil {
		t.Fatal(err)
	}

	want := "H4sIAAAAAAAA/ypJLS4x1CtJY6AhMDAwMDAzMADTBpg0JtvQwNTUjEHBgJaOgoHS4pLEIgYDiu1C99wQAaD4NxqM8W8+Gv+jYBSMglFASwAIAAD//+x/f2UACAAA"

	got := base64.StdEncoding.EncodeToString(buf.Bytes())

	if got != want {
		t.Errorf("got %s, wanted %s", got, want)
	}

	dest, err := ioutil.TempDir("", "test-tarball")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dest)

	gotFiles, err := Extract(buf, dest)
	if err != nil {
		t.Fatal(err)
	}

	wantFiles := 2
	if gotFiles != wantFiles {
		t.Errorf("wanted to extract %d files, but got %d", wantFiles, gotFiles)
	}
}
