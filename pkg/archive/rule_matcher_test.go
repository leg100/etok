package archive

import (
	"testing"

	"github.com/leg100/etok/pkg/testutil"
	"github.com/stretchr/testify/assert"
)

func TestRuleMatcher(t *testing.T) {
	testutil.Run(t, "without terraformignore", func(t *testutil.T) {
		// temp directory without .terraformignore
		tmpdir := t.NewTempDir().Write("foo.txt", []byte("foo"))
		matcher := newRuleMatcher(tmpdir.Root())
		if !assert.Equal(t, 3, len(matcher.rules)) {
			t.Fatal("A directory without .terraformignore should get the default patterns")
		}
	})

	testutil.Run(t, "with terraformignore", func(t *testutil.T) {
		matcher := newRuleMatcher("testdata/config-dir/")
		if !assert.Equal(t, 11, len(matcher.rules)) {
			t.Fatal("Expected to find rules from .terraformignore file")
		}
	})
}
