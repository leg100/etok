package archive

import (
	"path/filepath"
	"strings"

	"github.com/hashicorp/terraform-config-inspect/tfconfig"
	"k8s.io/klog/v2"
)

// On each iteration, parse the module for calls, add those calls to list of
// found modules, and then recurse over those modules.  Return the final list of
// all found modules.
func walk(path string) ([]string, error) {
	var found []string
	// parse module for calls
	mod, diag := tfconfig.LoadModule(path)
	if diag.HasErrors() {
		return nil, diag.Err()
	}
	for _, mc := range mod.ModuleCalls {
		// Ignore git://, https://, etc
		if !isLocalPath(mc.Source) {
			continue
		}

		// add call to list of found modules
		found = append(found, filepath.Join(path, mc.Source))
		klog.V(2).Infof("adding local module to archive: %s", filepath.Join(path, mc.Source))

		// lookup calls in newly found module and add them
		mods, err := walk(filepath.Join(path, mc.Source))
		if err != nil {
			return nil, err
		}
		found = append(found, mods...)
	}
	return found, nil
}

// Is a filesystem path and not a git, https path.
// See: https://www.terraform.io/docs/modules/sources.html#local-paths
func isLocalPath(path string) bool {
	if strings.HasPrefix(path, "./") || strings.HasPrefix(path, "../") {
		return true
	}
	return false
}
