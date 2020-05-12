package cmd

import (
	"testing"

	"github.com/leg100/stok/util/slice"
)

func TestDoubleDashArgsHandler(t *testing.T) {
	got := DoubleDashArgsHandler([]string{"stok", "plan", "--", "-no-color"})
	want := []string{"-no-color"}

	if !slice.IdenticalStrings(want, got) {
		t.Errorf("want %+v got %+v", want, got)
	}
}

func TestShellWrapDoubleDashArgsHandler(t *testing.T) {
	got := ShellWrapDoubleDashArgsHandler([]string{"stok", "shell", "--", "foo", "bar"})
	want := []string{"-c", "\"foo bar\""}

	if !slice.IdenticalStrings(want, got) {
		t.Errorf("want %+v got %+v", want, got)
	}
}
