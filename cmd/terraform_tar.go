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
	log.Debugf("found *.tf files: %v", filenames)

	tar, err := util.Create(wd, filenames)
	if err != nil {
		return nil, err
	}
	return tar, nil
}
