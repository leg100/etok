package path

import (
	"path/filepath"
	"testing"

	"github.com/leg100/etok/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCopy(t *testing.T) {
	dst := testutil.NewTempDir(t).Root()
	require.NoError(t, Copy("fixtures/src", dst))

	assert.DirExists(t, filepath.Join(dst, "a"))
	assert.DirExists(t, filepath.Join(dst, "b"))
	assert.DirExists(t, filepath.Join(dst, "c"))

	assert.FileExists(t, filepath.Join(dst, "a", "file"))
	assert.FileExists(t, filepath.Join(dst, "b", "file"))
	assert.FileExists(t, filepath.Join(dst, "c", "file"))
}
