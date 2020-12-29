package launcher

import "github.com/leg100/etok/pkg/util/slice"

// Commands that are enqueued onto a workspace queue.
var queueable = []string{
	"apply",
	"sh",
}

func IsQueueable(cmd string) bool {
	return slice.ContainsString(queueable, cmd)
}
