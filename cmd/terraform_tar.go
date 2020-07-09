package cmd

import (
	"bytes"
	"os"
	"path/filepath"

	"github.com/apex/log"
	"github.com/leg100/stok/util"
)

// Creates tarball from *.tf files found in 'path'
// TODO: unit test
// TODO: skip this (and the config file it's embedded in) if command
// doesn't need *.tf files (e.g. terraform import)
func (t *terraformCmd) createTar() (*bytes.Buffer, error) {
	if err := os.Chdir(t.Path); err != nil {
		return nil, err
	}
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	filenames, err := filepath.Glob("*.tf")
	if err != nil {
		return nil, err
	}

	tar, err := util.Create(wd, filenames)
	if err != nil {
		return nil, err
	}

	log.WithFields(log.Fields{
		"files": filenames,
		"bytes": tar.Len(),
	}).Debug("archive created")

	return tar, nil
}
