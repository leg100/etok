package cmd

import (
	"testing"

	"github.com/leg100/stok/util/slice"
)

func TestShellWrapArgs(t *testing.T) {
	got := shellWrapArgs([]string{"foo", "bar"})
	want := []string{"-c", "\"foo bar\""}

	if !slice.IdenticalStrings(want, got) {
		t.Errorf("want %+v got %+v", want, got)
	}
}
