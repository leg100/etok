package archive

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/apex/log"
)

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

// Create gzipped tarball of *.tf files in root directory
func Create(root string) ([]byte, error) {
	// Remember current directory so that we can switch back it later on
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	defer os.Chdir(wd)

	// Change into root directory, so that all paths within tarball are relative to it
	if err := os.Chdir(root); err != nil {
		return nil, err
	}

	paths, err := filepath.Glob("*.tf")
	if err != nil {
		return nil, err
	}

	out := new(bytes.Buffer)
	if err := CreateTarGz(out, paths); err != nil {
		return nil, err
	}

	// Check if max size exceeded
	if err := checkSize(out.Len()); err != nil {
		return nil, err
	}

	log.WithFields(log.Fields{"files": paths, "bytes": out.Len()}).Debug("archive created")
	return out.Bytes(), nil
}

func CreateTarGz(w io.Writer, paths []string) error {
	zw := gzip.NewWriter(w)
	defer zw.Close()

	return CreateTar(zw, paths)
}

func CreateTar(w io.Writer, paths []string) error {
	tw := tar.NewWriter(w)
	defer tw.Close()

	for _, f := range paths {
		data, err := ioutil.ReadFile(f)
		if err != nil {
			return err
		}

		hdr := &tar.Header{
			Name: f,
			Mode: 0600,
			Size: int64(len(data)),
		}

		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}

		if _, err := tw.Write(data); err != nil {
			return err
		}
	}
	return nil
}

// Untar gzipped tarball src to dest directory
func Extract(src, dest string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()

	zr, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer zr.Close()

	var files int
	tr := tar.NewReader(zr)
	for {
		hdr, err := tr.Next()

		if err == io.EOF {
			break // End of archive
		}

		if err != nil {
			return err
		}

		path := filepath.Join(dest, hdr.Name)
		f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0644)
		if err != nil {
			return err
		}
		if _, err := io.Copy(f, tr); err != nil {
			return err
		}
		if err := f.Close(); err != nil {
			return err
		}
		files++
	}

	log.WithFields(log.Fields{"files": files, "path": dest}).Debug("extracted tarball")
	return nil
}
