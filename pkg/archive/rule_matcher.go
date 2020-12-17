package archive

import (
	"fmt"
	"os"
	"path/filepath"

	"k8s.io/klog/v2"
)

const ignoreFile = ".terraformignore"

// ruleMatcher checks whether paths match ignore rules
type ruleMatcher struct {
	// rules determining whether path should be ignored or not
	rules []rule
	// base directory that rules are relative to
	base string
}

// newRuleMatcher creates a new ruleMatcher, setting the rules and the base
// directory according to whether it finds a .terraformignore file. The file is
// first checked in path, and if not found there, it is checked in parent
// directories recursively. If found, the rules are parsed from the file and the
// base directory is set to the directory where the file is found. If not found,
// a default set of rules applies to original path parameter.
func newRuleMatcher(path string) *ruleMatcher {
	file, err := findIgnoreFile(path)
	if err != nil {
		// If there's any kind of file error, punt and use the default ignore
		// patterns
		if !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "Error reading .terraformignore, default exclusions will apply: %v \n", err)
		}
		klog.V(1).Infof("ignore file not found, default exclusions apply to: %s", path)
		return &ruleMatcher{rules: defaultExclusions, base: path}
	}
	defer file.Close()
	klog.V(1).Infof("found ignore file: %s", file.Name())
	return &ruleMatcher{rules: readRules(file), base: filepath.Dir(file.Name())}
}

func (rm *ruleMatcher) match(path string, isDir bool) (matched bool, err error) {
	if isDir {
		// Catch directories so we don't end up with empty directories
		return matchIgnoreRule(path+string(os.PathSeparator), rm.rules), nil
	}

	path, err = filepath.Rel(rm.base, path)
	if err != nil {
		return false, nil
	}
	if path == "." {
		// Always ignore empty paths
		return true, nil
	}

	return matchIgnoreRule(path, rm.rules), nil
}

// searchFileInAncestors checks for existence of filename at the given path, and
// if not found, checks the path's parent directories recursively. If successful
// the path to the filename is returned, otherwise an empty string is returned.
// Any other error encountered during the search is returned in error.
func findIgnoreFile(path string) (*os.File, error) {
	fileinfo, err := os.Stat(filepath.Join(path, ignoreFile))
	if os.IsNotExist(err) || fileinfo.IsDir() {
		parent := filepath.Dir(path)
		if parent == path {
			// Reached root (or c:\ ?) without finding filename
			return nil, err
		}
		return findIgnoreFile(parent)
	}
	if err != nil {
		// Some error other than not found
		return nil, err
	}
	return os.Open(filepath.Join(path, ignoreFile))
}
