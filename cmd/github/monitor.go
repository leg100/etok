package github

import "time"

// A process is runnable and invokes github operations
type process interface {
	start() error
	githubOperation
}

// A task queue receives github operations and invokes them
type taskQueue interface {
	send(githubOperation)
}

// Monitor runs a process and periodically sends it to a task queue on a regular
// interval until it finishes.
func monitor(queue taskQueue, proc process, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	errch := make(chan error)
	go func() {
		errch <- proc.start()
	}()

	for {
		select {
		case <-ticker.C:
			// Periodically send updates about run
			queue.send(proc)
		case <-errch:
			// Send final update upon completion
			queue.send(proc)
			return
		}
	}
}
