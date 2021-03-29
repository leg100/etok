package archive

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/leg100/etok/pkg/testutil"
	"github.com/leg100/etok/pkg/util/path"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestArchive(t *testing.T) {
	// Create archive with path to the root module (m0)
	arc, err := NewArchive("testdata/config-dir/m0")
	require.NoError(t, err)

	// Add module references to archive
	require.NoError(t, arc.Walk())

	// Pack archive into compressed tarball
	w := new(bytes.Buffer)
	meta, err := arc.Pack(w)
	require.NoError(t, err)

	assert.NotZero(t, meta.Size)
	assert.NotZero(t, meta.CompressedSize)

	// De-compress
	gr, err := gzip.NewReader(w)
	require.NoError(t, err)

	// Extract
	tr := tar.NewReader(gr)

	var files []string
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}

		files = append(files, hdr.Name)
	}

	// Assert that it has compiled not only the root module's (m0) files and
	// subdirectories, but those of other referenced local  modules too (m1, m2,
	// and m3).
	assert.Equal(t, []string{
		"m0/",
		"m0/.terraform/modules/",
		"m0/.terraform/modules/README",
		"m0/bar.txt",
		"m0/exe",
		"m0/foo.terraform/",
		"m0/foo.terraform/bar.txt",
		"m0/m2/",
		"m0/m2/main.tf",
		"m0/m3/",
		"m0/m3/main.tf",
		"m0/main.tf",
		"m0/sub/",
		"m0/sub/zip.txt",
		"m1/",
		"m1/globals.tf",
		"m1/main.tf",
	}, files)
}

func TestMaxSize(t *testing.T) {
	tmpdir := testutil.NewTempDir(t).Chdir().Mkdir(".git").WriteRandomFile("toobig", MaxConfigSize+1)

	arc, err := NewArchive(tmpdir.Root(), MaxSize(MaxConfigSize))
	require.NoError(t, err)

	_, err = arc.Pack(new(bytes.Buffer))
	if assert.Error(t, err) {
		assert.True(t, errors.Is(err, MaxSizeError(MaxConfigSize)))
	}
}

func TestWalk(t *testing.T) {
	arc, err := NewArchive("testdata/config-dir/m0")
	require.NoError(t, err)

	require.NoError(t, arc.Walk())

	got, err := path.RelToWorkingDir(arc.mods)
	require.NoError(t, err)

	// Module walk is non-deterministic
	sort.Strings(got)

	want := []string{
		"testdata/config-dir/m0",
		"testdata/config-dir/m0/m2",
		"testdata/config-dir/m0/m3",
		"testdata/config-dir/m1",
	}

	assert.Equal(t, want, got)
}

func TestUnpack(t *testing.T) {
	// Create archive with path to the root module (m0)
	arc, err := NewArchive("testdata/config-dir/m0")
	require.NoError(t, err)

	// Add module references to archive
	require.NoError(t, arc.Walk())

	// Pack archive into compressed tarball
	tarball := new(bytes.Buffer)
	meta, err := arc.Pack(tarball)
	require.NoError(t, err)

	assert.NotZero(t, meta.Size)
	assert.NotZero(t, meta.CompressedSize)

	dst := testutil.NewTempDir(t)

	require.NoError(t, Unpack(tarball, dst.Root()))

	var got []string
	filepath.Walk(dst.Root(), func(path string, info os.FileInfo, err error) error {
		path, err = filepath.Rel(dst.Root(), path)
		require.NoError(t, err)
		got = append(got, path)
		return nil
	})

	assert.Equal(t, []string{
		".",
		"m0",
		"m0/.terraform",
		"m0/.terraform/modules",
		"m0/.terraform/modules/README",
		"m0/bar.txt",
		"m0/exe",
		"m0/foo.terraform",
		"m0/foo.terraform/bar.txt",
		"m0/m2",
		"m0/m2/main.tf",
		"m0/m3",
		"m0/m3/main.tf",
		"m0/main.tf",
		"m0/sub",
		"m0/sub/zip.txt",
		"m1",
		"m1/globals.tf",
		"m1/main.tf"}, got)
}
