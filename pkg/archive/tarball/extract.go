package tarball

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/leg100/stok/pkg/log"
)

// Extract compressed tarball to dest directory
func Extract(src, dest string) (int, error) {
	bytes, err := ioutil.ReadFile(src)
	if err != nil {
		return 0, err
	}
	return ExtractBytes(bytes, dest)
}

// extractBytes extracts compressed tarball to dest path.
func ExtractBytes(tarball []byte, dest string) (files int, err error) {
	buf := bytes.NewBuffer(tarball)

	zr, err := gzip.NewReader(buf)
	if err != nil {
		return files, err
	}
	defer zr.Close()

	tr := tar.NewReader(zr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break // End of archive
		}
		if err != nil {
			return files, err
		}

		target := filepath.Join(dest, hdr.Name)

		switch hdr.Typeflag {
		case tar.TypeDir:
			if _, err := os.Stat(target); err != nil {
				if err := os.MkdirAll(target, 0755); err != nil {
					return files, err
				}
			}
		case tar.TypeReg:
			// Ignore mode in tarball
			f, err := os.OpenFile(target, os.O_RDWR|os.O_CREATE, 0644)
			if err != nil {
				return files, err
			}
			if _, err := io.Copy(f, tr); err != nil {
				return files, err
			}
			if err := f.Close(); err != nil {
				return files, err
			}
		}
		files++
	}

	log.Debugf("extracted %d files to: %s\n", files, dest)
	return files, err
}
