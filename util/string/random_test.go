package random

import (
	"regexp"
	"testing"
)

func TestRandomString(t *testing.T) {
	got := RandomString(16)
	want := regexp.MustCompile(`[A-Za-z]{16}`)

	if !want.MatchString(got) {
		t.Errorf("want %s, got %s\n", want, got)
	}
}
