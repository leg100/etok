package archive

import (
	"path/filepath"

	"github.com/bmatcuk/doublestar"
	"github.com/leg100/stok/pkg/log"
)

func matchIgnoreRules(path string) bool {
	var matched bool
	path = filepath.FromSlash(path)
	for _, rule := range defaultExclusions {
		match, _ := doublestar.PathMatch(rule.val, path)
		if match {
			matched = !rule.excluded
		}
	}

	if matched {
		log.Debugf("Skipping excluded path: %s\n", path)
	}

	return matched
}

/*
	Default rules as they would appear in .terraformignore:
	.git/
	.terraform/
	!.terraform/modules/
*/

var defaultExclusions = []rule{
	{
		val:      filepath.Join("**", ".git", "**"),
		excluded: false,
	},
	{
		val:      filepath.Join("**", ".terraform", "**"),
		excluded: false,
	},
	{
		val:      filepath.Join("**", ".terraform", "modules", "**"),
		excluded: true,
	},
}

type rule struct {
	val      string // the value of the rule itself
	excluded bool   // ! is present, an exclusion rule
}
