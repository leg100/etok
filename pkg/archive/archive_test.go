package archive

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"sort"
	"testing"

	"github.com/leg100/etok/testutil"
	"github.com/leg100/etok/util/path"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestArchive(t *testing.T) {
	arc, err := NewArchive("testdata/modtree/root/mod")
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

	assert.Equal(t, []string{
		"root/mod/.terraformignore",
		"root/mod/.terraformrc",
		"root/mod/bar.txt",
		"root/mod/exe",
		"root/mod/foo.terraform/",
		"root/mod/foo.terraform/bar.txt",
		"root/mod/inner/",
		"root/mod/inner/mods/",
		"root/mod/inner/mods/m2/",
		"root/mod/inner/mods/m2/main.tf",
		"root/mod/inner/mods/m3/",
		"root/mod/inner/mods/m3/main.tf",
		"root/mod/main.tf",
		"root/mod/sub/",
		"root/mod/sub/zip.txt",
		"outer/mods/m1/globals.tf",
		"outer/mods/m1/main.tf",
	}, files)
}

func TestMaxSize(t *testing.T) {
	tmpdir := testutil.NewTempDir(t).Chdir().WriteRandomFile("toobig", MaxConfigSize+1)

	arc, err := NewArchive(tmpdir.Root(), MaxSize(MaxConfigSize))
	require.NoError(t, err)

	_, err = arc.Pack(new(bytes.Buffer))
	if assert.Error(t, err) {
		assert.True(t, errors.Is(err, MaxSizeError(MaxConfigSize)))
	}
}

func TestWalk(t *testing.T) {
	arc, err := NewArchive("testdata/modtree/root/mod")
	require.NoError(t, err)

	require.NoError(t, arc.Walk())

	got, err := path.RelToWorkingDir(arc.mods)
	require.NoError(t, err)

	// Module walk is non-deterministic
	sort.Strings(got)

	want := []string{
		"testdata/modtree/outer/mods/m1",
		"testdata/modtree/root/mod",
		"testdata/modtree/root/mod/inner/mods/m2",
		"testdata/modtree/root/mod/inner/mods/m3",
	}

	assert.Equal(t, want, got)
}
