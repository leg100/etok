package path

import (
	"os"
	"path/filepath"
	"strings"
)

// Utility functions for working with filesystem paths

// Find a common prefix amongst paths
func CommonPrefix(paths []string) (string, error) {
	switch len(paths) {
	case 0:
		return "", nil
	case 1:
		return paths[0], nil
	}

	// Clean up first path and ensure it is absolute
	c, err := EnsureAbs(filepath.Clean(paths[0]))
	if err != nil {
		return "", err
	}

	// Add trailing sep to path
	c += string(os.PathSeparator)

	for _, v := range paths[1:] {
		// Clean up path and ensure it is absolute
		var err error
		v, err = EnsureAbs(filepath.Clean(v))
		if err != nil {
			return "", err
		}

		// Find first non-common byte and truncate c
		if len(v) < len(c) {
			for i := 0; i < len(c); i++ {
				if v[i] != c[i] {
					c = c[:i]
					break
				}
			}
		}

		// Remove trailing non-separator characters
		for i := len(c) - 1; i >= 0; i-- {
			if c[i] == os.PathSeparator {
				c = c[:i]
				break
			}
		}
	}

	return c, nil
}

// MakeRelativeTo makes all paths relative to base
// func MakeRelativeTo(base string, paths []string) ([]string, error) {
// 	for _, p := range paths {
// 		var err error
// 		p, err = filepath.Rel
// 		if err != nil {
// 			return nil, err
// 		}
// 	}
// 	return paths, nil
// }

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
