package tarball

import (
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/leg100/stok/testutil"
	"github.com/stretchr/testify/require"
	"gotest.tools/assert"
)

func TestExtract(t *testing.T) {
	testutil.Run(t, "", func(t *testutil.T) {
		// Dest dir for extracting tarball to
		dest := t.NewTempDir().Root()
		// Path to which tarball will be written
		tarballPath := filepath.Join(t.NewTempDir().Root(), "tarball.tar.gz")

		// Create tarball bytes
		bytes, err := Create("../fixtures/modtree", paths, MaxConfigSize)
		require.NoError(t, err)

		// Write tarball to file on disk
		require.NoError(t, ioutil.WriteFile(tarballPath, bytes, 0644))

		// Extract tarball from disk, and write to dest dir
		files, err := Extract(tarballPath, dest)
		require.NoError(t, err)
		assert.Equal(t, len(paths), files)
	})
}
