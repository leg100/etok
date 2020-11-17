package archive

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/leg100/stok/pkg/archive/modwalker"
	"github.com/leg100/stok/pkg/archive/tarball"
	"github.com/leg100/stok/util/path"
)

// Archive creates a compressed tarball containing not only the root module
// but local module calls too, including transitive calls. Returns the contents
// of the tarball and the relative path to the root module within the tarball.
func Archive(root string) ([]byte, string, error) {
	// Retrieve module paths (relative to PWD)
	mods, err := modwalker.Walk(root)
	if err != nil {
		return nil, "", err
	}

	// Ensure root mod is absolute path (and clean too)
	root, err = path.EnsureAbs(root)
	if err != nil {
		return nil, "", err
	}

	// Make all mods absolute (and clean them too)
	mods, err = path.EnsureAllAbs(mods)
	if err != nil {
		return nil, "", err
	}

	// Add root mod to mods
	mods = append(mods, root)

	// TODO: might need to ensure all paths have a trailing slash

	// Now, prefix might be the path to a module, or the parent directory of two or more modules.
	// Either way, we need the list of these 'parent' modules, because it is these modules
	// that will be walked, adding their files and subdirs to the tarball.
	mods = path.RemoveNestedPaths(mods)

	// Get common path prefix shared by modules
	prefix, err := path.CommonPrefix(mods)

	// Walk mod dirs, building a list of paths to be included in the archive
	paths, err := walkMods(prefix, mods)

	// Create tarball of all modules
	bytes, err := tarball.Create(prefix, paths, tarball.MaxConfigSize)
	if err != nil {
		return nil, "", err
	}

	// Get relative path to root module from the common prefix directory
	relPathToRoot, err := filepath.Rel(prefix, root)
	if err != nil {
		return nil, "", err
	}

	return bytes, relPathToRoot, nil
}

func walkMods(base string, mods []string) (paths []string, err error) {
	for _, m := range mods {
		err = filepath.Walk(m, func(path string, info os.FileInfo, err error) error {
			// Get the relative path from the current module directory.
			subpath, err := filepath.Rel(base, path)
			if err != nil {
				return fmt.Errorf("Failed to get relative path for file %q: %v", path, err)
			}
			if subpath == "." {
				return nil
			}

			if matchIgnoreRules(subpath) {
				return nil
			}

			// Catch directories so we don't end up with empty directories,
			// the files are ignored correctly
			if info.IsDir() {
				if matchIgnoreRules(subpath + string(os.PathSeparator)) {
					return nil
				}
			}

			paths = append(paths, subpath)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return
}
