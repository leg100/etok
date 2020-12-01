package version

import (
	"fmt"
)

var (
	Version = "unknown"
	Commit  = "unknown"
	Image   = "leg100/etok:" + Version
)

func PrintableVersion() string {
	return fmt.Sprintf("%s\t%s\n", Version, Commit)
}
