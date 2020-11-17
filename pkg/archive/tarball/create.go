package tarball

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/leg100/stok/pkg/log"
)

// Create creates a gzipped tarball. The paths are expected to be relative to
// the base directory.  During creation if the size of the tarball exceeds
// maxSize an error is returned.
func Create(base string, paths []string, maxSize int) ([]byte, error) {
	w := new(bytes.Buffer)
	zw := gzip.NewWriter(w)
	tw := tar.NewWriter(zw)

	for _, fpath := range paths {
		fstat, err := os.Stat(filepath.Join(base, fpath))
		if err != nil {
			return nil, err
		}

		hdr, err := tar.FileInfoHeader(fstat, fpath)
		if err != nil {
			return nil, err
		}
		// hdr.Name is only the basename so overwrite with path
		hdr.Name = fpath

		if err := tw.WriteHeader(hdr); err != nil {
			return nil, err
		}

		if !fstat.IsDir() {
			data, err := os.Open(filepath.Join(base, fpath))
			if err != nil {
				return nil, err
			}
			if _, err := io.Copy(tw, data); err != nil {
				return nil, err
			}
			// Check if max size exceeded
			if err := checkSize(w.Len()); err != nil {
				return nil, err
			}
		}
	}

	if err := tw.Close(); err != nil {
		return nil, err
	}

	if err := zw.Close(); err != nil {
		return nil, err
	}

	log.Debugf("archived %d files totalling %d bytes\n", len(paths), w.Len())
	return w.Bytes(), nil
}

// ConfigMap/etcd only supports data payload of up to 1MB, which limits the size of the config that
// can be can be uploaded (after compression).
// https://github.com/kubernetes/kubernetes/issues/19781
const MaxConfigSize = 1024 * 1024

func checkSize(size int) error {
	if size > MaxConfigSize {
		return fmt.Errorf("max config size exceeded: %d > %d", size, MaxConfigSize)
	}
	return nil
}
