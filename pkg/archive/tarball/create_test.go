package tarball

import (
	"testing"

	"github.com/leg100/stok/testutil"
	"github.com/stretchr/testify/require"
	"gotest.tools/assert"
)

var paths = []string{
	"./root/",
	"./root/mod/",
	"./root/mod/main.tf",
	"./root/mod/inner/",
	"./root/mod/inner/mods/",
	"./root/mod/inner/mods/m2/",
	"./root/mod/inner/mods/m2/main.tf",
	"./root/mod/inner/mods/m3/",
	"./root/mod/inner/mods/m3/main.tf",
	"./outer/",
	"./outer/mods/",
	"./outer/mods/m1/",
	"./outer/mods/m1/main.tf",
}

func TestCreate(t *testing.T) {
	testutil.Run(t, "create and extract", func(t *testutil.T) {
		targz, err := Create("../fixtures/modtree", paths, MaxConfigSize)
		require.NoError(t, err)

		dest := t.NewTempDir()

		files, err := ExtractBytes(targz, dest.Root())
		require.NoError(t, err)
		assert.Equal(t, len(paths), files)
	})

	// Trigger max size error. Must account for (a) compression and (b) flushing, so opted for 10mb
	// (10x the limit). Because we cannot rely on the OS to flush the buffer this test may not be
	// 100% reliable...
	testutil.Run(t, "trigger max size error", func(t *testutil.T) {
		tmpdir := t.NewTempDir().Chdir().WriteRandomFile("toobig.tf", 1024*1024*10)

		_, err := Create(tmpdir.Root(), []string{"./toobig.tf"}, MaxConfigSize)
		require.Error(t, err)
	})

	// // Check that the max size error is not triggered when the size is under the limit
	testutil.Run(t, "no max size error", func(t *testutil.T) {
		tmpdir := t.NewTempDir().Chdir().WriteRandomFile("nottoobig.tf", 1024*512)

		_, err := Create(tmpdir.Root(), []string{"./nottoobig.tf"}, MaxConfigSize)
		require.NoError(t, err)
	})
}
