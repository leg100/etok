package path

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

// Utility functions for working with filesystem paths

// EnsureAllAbs ensures all paths are absolute paths
func EnsureAllAbs(paths []string) (updated []string, err error) {
	for _, p := range paths {
		p, err = EnsureAbs(p)
		if err != nil {
			return nil, err
		}
		updated = append(updated, p)
	}
	return updated, nil
}

// EnsureAbs ensures path is absolute, if not already.
func EnsureAbs(path string) (string, error) {
	if !filepath.IsAbs(path) {
		return filepath.Abs(path)
	}
	return path, nil
}

// RelToWorkingDir turns all absolute paths into relative paths relative to the
// current working directory.
func RelToWorkingDir(paths []string) (updated []string, err error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	for _, p := range paths {
		rel, err := filepath.Rel(wd, p)
		if err != nil {
			return nil, err
		}
		updated = append(updated, rel)
	}
	return updated, nil
}

// Remove paths which are nested within another path
func RemoveNestedPaths(paths []string) (unnested []string) {
	for _, p := range paths {
		// Create a new list of unnested paths on every iteration
		unnested = updateUnnested(p, unnested)
	}
	return unnested
}

// Check candidate path against list of paths to see if it is nested
// within any of them, and update accordingly: if candidate is deemed
// unnested then add it to the list; if candidate nests a path, then remove
// path from list.
func updateUnnested(c string, paths []string) (updated []string) {
	for _, p := range paths {
		if c == p {
			// Candidate is identical to a path so it cannot be nested
			// within any other path, and is thus deemed unnested too
			return append(paths, p)
		}
		if strings.HasPrefix(c, p) {
			// Candidate has path as its prefix, so candidate must be
			// nested; return paths un-updated
			return paths
		}

		if strings.HasPrefix(p, c) {
			// Path has candidate as its prefix, so path can no longer
			// be deemed unnested, so leave it out of updated list.
			// Candidate might be nesting other paths so keep
			// on checking other paths
			continue
		}
		// Keep path in updated list
		updated = append(updated, p)
	}
	// Candidate was not found to be nested within paths,
	// so add it to updated list of unnested paths.
	return append(updated, c)
}

// Copy directory recursively
func Copy(src, dst string) error {
	src = filepath.Clean(src)

	// Always ensure dest dir is created
	os.MkdirAll(dst, 0755)

	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		path = strings.Replace(path, src, "", 1)

		if info.IsDir() {
			return os.MkdirAll(filepath.Join(dst, path), 0755)
		}

		data, _ := ioutil.ReadFile(filepath.Join(src, path))
		return ioutil.WriteFile(filepath.Join(dst, path), data, 0644)
	})
}
