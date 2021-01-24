package launcher

import "github.com/leg100/etok/pkg/util/slice"

// Commands that are enqueued onto a workspace queue.
var queueable = []string{
	"apply",
	"destroy",
	"force-unlock",
	"import",
	"init",
	"refresh",
	"state mv",
	"state push",
	"state replace-provider",
	"state rm",
	"taint",
	"untaint",
	"sh",
}

func IsQueueable(cmd string) bool {
	return slice.ContainsString(queueable, cmd)
}
