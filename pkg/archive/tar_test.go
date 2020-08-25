package archive

import (
	"encoding/base64"
	"testing"

	"github.com/leg100/stok/testutil"
	"github.com/stretchr/testify/require"
	"gotest.tools/assert"
)

func TestCreate(t *testing.T) {
	testutil.Run(t, "", func(t *testutil.T) {
		targz, err := Create("fixtures/tarball")
		require.NoError(t, err)

		want := "H4sIAAAAAAAA/ypJLS4x1CtJY6AhMDAwMDAzMADTBpg0JtvQwNTUjEHBgJaOgoHS4pLEIgYDiu1C99wQAaD4NxqM8W8+Gv+jYBSMglFASwAIAAD//+x/f2UACAAA"
		require.Equal(t, want, base64.StdEncoding.EncodeToString(targz))
	})

	// Trigger max size error. Must account for (a) compression and (b) flushing, so opted for 10mb
	// (10x the limit). Because we cannot rely on the OS to flush the buffer this test may not be
	// 100% reliable...
	testutil.Run(t, "MaxSizeError", func(t *testutil.T) {
		tmpdir := t.NewTempDir().Chdir().WriteRandomFile("toobig.tf", 1024*1024*10)

		_, err := Create(tmpdir.Root())
		require.Error(t, err)
	})

	// Check that the max size error is not triggered when the size is under the limit
	testutil.Run(t, "NoMaxSizeError", func(t *testutil.T) {
		tmpdir := t.NewTempDir().Chdir().WriteRandomFile("nottoobig.tf", 1024*512)

		_, err := Create(tmpdir.Root())
		require.NoError(t, err)
	})
}

func TestExtract(t *testing.T) {
	testutil.Run(t, "", func(t *testutil.T) {
		dest := t.NewTempDir()

		files, err := Extract("fixtures/tarball.tar.gz", dest.Root())
		require.NoError(t, err)
		assert.Equal(t, 2, files)
	})
}
