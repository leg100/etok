package archive

import (
	"path/filepath"
	"strings"

	"github.com/hashicorp/terraform-config-inspect/tfconfig"
)

// GetModules constructs a tree data structure representing all local
// terraform modules to be archived. From the root terraform module
// path it walks all local modules calls and returns a tree node that
// represents a directory containing all those local modules.
func GetModules(root string) ([]string, error) {
	dir, err := (&directory{path: root}).parseMod()
	if err != nil {
		return nil, err
	}
	return dir.base().Modules(), nil
}

type directory struct {
	// Absolute path
	path     string
	parent   *directory
	children []*directory
}

// GetRelativePathsToModules gets the relative paths of modules,
// relative to the path of the receiver node
func (d *directory) getRelativePathsToModules() ([]string, error) {
	var relPaths []string
	for _, m := range d.Modules() {
		rel, err := filepath.Rel(d.path, m)
		if err != nil {
			return nil, err
		}
		relPaths = append(relPaths, rel)
	}
	return relPaths, nil
}

// Modules returns a list of absolute paths of terraform modules
// in a depth-first search of the tree beginning with the node
// of the receiver.
func (d *directory) Modules() (modules []string) {
	// Add self first if module
	if d.isModule {
		modules = append(modules, d.path)
	}
	// Then recursively perform DFS of children
	return append(modules, d.modules()...)
}

func (d *directory) modules() (modules []string) {
	for _, child := range d.children {
		if child.isModule {
			modules = append(modules, child.path)
		}
		modules = append(modules, child.modules()...)
	}
	return modules
}

// Parse info about the terraform module in this directory, and walk calls to other modules
func (d *directory) parseMod() (*directory, error) {
	// Obtain information about module
	mod, diag := tfconfig.LoadModule(d.path)
	if diag.HasErrors() {
		return nil, diag.Err()
	}
	for _, mc := range mod.ModuleCalls {
		// Ignore git://, https://, etc
		if !isLocalPath(mc.Source) {
			continue
		}
		d.walk(mc.Source)
	}

	return d, nil
}

// Walk the relative path to the target module, adding each component
// as a parent or child accordingly.
func (d *directory) walk(relPath string) {
	parts := strings.Split(relPath, "/")

	switch parts[0] {
	case "":
		// This is the target module
		d.parseMod()
	case ".":
		// Strip leading './'
		d.walk(strings.Join(parts[1:], "/"))
	case "..":
		// Add parent node and walk further nodes
		d.addParent().walk(strings.Join(parts[1:], "/"))
	default:
		// Add child node and walk further nodes
		d.addChild(parts[0]).walk(strings.Join(parts[1:], "/"))
	}
}

// Add a parent node, adding self as child to the node (the tree is undirected,
// traversible up or down). Returns parent node.
func (d *directory) addParent() *directory {
	if d.parent == nil {
		parent := &directory{
			path:     filepath.Join(d.path, ".."),
			children: []*directory{d},
		}
		d.parent = parent
	}
	return d.parent
}

// Add a child node and return it
func (d *directory) addChild(basename string) *directory {
	child, exists := d.getChild(basename)
	if !exists {
		child = &directory{
			path:   filepath.Join(d.path, basename),
			parent: d,
		}
		d.children = append(d.children, child)
	}
	return child
}

// Get the base of the tree (not to be confused with the terraform root module, nor the unix root
// directory!)
func (d *directory) base() *directory {
	if d.parent != nil {
		return d.parent.base()
	} else {
		return d
	}
}

// Get existing child with given basename if it exists
func (d *directory) getChild(basename string) (*directory, bool) {
	for _, c := range d.children {
		if filepath.Base(c.path) == basename {
			return c, true
		}
	}
	return nil, false
}

// Is a filesystem path and not a git, https path.
// See: https://www.terraform.io/docs/modules/sources.html#local-paths
func isLocalPath(path string) bool {
	if strings.HasPrefix(path, "./") || strings.HasPrefix(path, "../") {
		return true
	}
	return false
}
