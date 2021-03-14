package github

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type fakeTaskQueue struct{}

func (q *fakeTaskQueue) send(op githubOperation) {
	op.invoke(nil)
}

type fakeProcess struct {
	counter int
	queue   chan int
}

// Run process for at least 10ms to simulate it doing something
func (p *fakeProcess) run() error {
	time.Sleep(time.Millisecond * 10)
	return nil
}

// Check invoke is being called by sending a counter to a queue, which the test
// func can receive.
func (p *fakeProcess) invoke(client *GithubClient) error {
	p.queue <- p.counter
	p.counter++
	return nil
}

// Test monitor can run a process obj and send it to a task queue on a regular
// interval.
func TestMonitor(t *testing.T) {
	proc := &fakeProcess{
		queue: make(chan int, 100),
	}
	monitor(&fakeTaskQueue{}, proc, time.Millisecond)

	// Receive expected counters demonstrating monitor is sending updates
	assert.Equal(t, 0, <-proc.queue)
	assert.Equal(t, 1, <-proc.queue)
	assert.Equal(t, 2, <-proc.queue)
	assert.Equal(t, 3, <-proc.queue)
}
