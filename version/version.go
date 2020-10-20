package version

import (
	"fmt"
)

var (
	Version = "unknown"
	Commit  = "unknown"
	Image   = "leg100/stok:" + Version
)

func PrintableVersion() string {
	return fmt.Sprintf("%s\t%s\n", Version, Commit)
}
