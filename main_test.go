package main

import (
	"bytes"
	"regexp"
	"testing"
)

func TestMain(t *testing.T) {
	if is_local([]string{"plan"}) {
		t.Fatal("plan subcommand unexpectedly designated as local")
	}

	if !is_local([]string{"version"}) {
		t.Fatal("version subcommand unexpectedly designated as remote")
	}

	out = new(bytes.Buffer)
	if err := runLocal([]string{"version"}); err != nil {
		t.Error(err)
	}

	want := regexp.MustCompile(`^Terraform v0\.1`)
	got := out.(*bytes.Buffer).String()
	if !want.MatchString(got) {
		t.Errorf("wanted stdout to match %s but got %s\n", want, got)
	}
}
