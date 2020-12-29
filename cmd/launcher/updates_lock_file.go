package launcher

import "github.com/leg100/etok/pkg/util/slice"

// Commands that update the lock file (.terraform.lock.hcl)
var updateLockFile = []string{
	"init",
	"providers lock",
}

func UpdatesLockFile(cmd string) bool {
	return slice.ContainsString(updateLockFile, cmd)
}
