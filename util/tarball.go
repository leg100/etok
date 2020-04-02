package util

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
)

// corollary of Extract
func Create(dir string, filenames []string) (*bytes.Buffer, error) {
	b := new(bytes.Buffer)

	tw := tar.NewWriter(b)

	for _, f := range filenames {
		path := filepath.Join(dir, f)

		data, err := ioutil.ReadFile(path)
		if err != nil {
			return nil, err
		}

		hdr := &tar.Header{
			Name: f,
			Mode: 0600,
			Size: int64(len(data)),
		}

		if err := tw.WriteHeader(hdr); err != nil {
			return nil, err
		}

		if _, err := tw.Write(data); err != nil {
			return nil, err
		}
	}

	if err := tw.Close(); err != nil {
		return nil, err
	}

	bb := new(bytes.Buffer)
	zw := gzip.NewWriter(bb)

	_, err := zw.Write(b.Bytes())
	if err != nil {
		return nil, err
	}

	if err := zw.Close(); err != nil {
		return nil, err
	}

	return bb, nil
}

// corollary of Create
func Extract(r io.Reader, dest string) (int, error) {
	var files int

	zr, err := gzip.NewReader(r)
	if err != nil {
		return 0, err
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

		path := filepath.Join(dest, hdr.Name)
		f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0644)
		if err != nil {
			return files, err
		}
		if _, err := io.Copy(f, tr); err != nil {
			return files, err
		}
		if err := f.Close(); err != nil {
			return files, err
		}
		files++
	}
	return files, nil
}
