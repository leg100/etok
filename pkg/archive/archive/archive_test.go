package archive

import (
	"testing"

	"github.com/leg100/stok/pkg/archive/tarball"
	"github.com/leg100/stok/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestArchive(t *testing.T) {
	bytes, root, err := Archive("../fixtures/modtree/root/mod")
	require.NoError(t, err)
	assert.Equal(t, "root/mod", root)

	// Dest dir for extracting tarball to
	dest := testutil.NewTempDir(t).Root()

	num, err := tarball.ExtractBytes(bytes, dest)
	require.NoError(t, err)
	assert.Equal(t, 13, num)
}
