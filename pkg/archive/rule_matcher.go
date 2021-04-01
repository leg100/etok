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

// newRuleMatcher creates a new ruleMatcher, setting the rules according to
// whether it finds a .terraformignore file. The file is checked in
// <path>/.terraformignore. If found, the rules are parsed from the file.  If
// not found, a default set of rules apply.
func newRuleMatcher(path string) *ruleMatcher {
	file, err := os.Open(filepath.Join(path, ignoreFile))
	defer file.Close()

	if err != nil {
		if os.IsNotExist(err) {
			klog.V(1).Infof("ignore file not found, default exclusions apply to: %s", path)
		} else {
			// If there's any other kind of file error, punt and use the default
			// ignore patterns
			fmt.Fprintf(os.Stderr, "Error reading .terraformignore, default exclusions will apply: %v \n", err)
		}
		return &ruleMatcher{rules: defaultExclusions, base: path}
	}

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
