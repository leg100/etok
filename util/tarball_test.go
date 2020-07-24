package util

import (
	"bytes"
	"encoding/base64"
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTarball(t *testing.T) {
	targz, err := Create("fixtures/tarball", []string{"test1.tf", "test2.tf"})
	if err != nil {
		t.Fatal(err)
	}

	want := "H4sIAAAAAAAA/ypJLS4x1CtJY6AhMDAwMDAzMADTBpg0JtvQwNTUjEHBgJaOgoHS4pLEIgYDiu1C99wQAaD4NxqM8W8+Gv+jYBSMglFASwAIAAD//+x/f2UACAAA"

	got := base64.StdEncoding.EncodeToString(targz)
	require.Equal(t, want, got)

	dest, err := ioutil.TempDir("", "test-tarball")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dest)

	gotFiles, err := Extract(bytes.NewBuffer(targz), dest)
	if err != nil {
		t.Fatal(err)
	}

	require.Equal(t, 2, gotFiles)
}
