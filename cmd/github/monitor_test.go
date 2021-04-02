package github

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// Rather convoluted unit test for the process monitor.

type fakeTaskQueue struct{}

// Mock the task queue perform github client operations by calling proc's invoke
// method.
func (q *fakeTaskQueue) send(op githubOperation) {
	op.invoke(nil)
}

type fakeProcess struct {
	sync    chan struct{}
	updates chan int
	counter int
}

// Fake a process doing something by sending 5 things on an unbuffered channel
// and only once they're taken off the queue, exit, triggering the monitor to
// exit too.
func (p *fakeProcess) start() error {
	for i := 0; i < 5; i++ {
		p.sync <- struct{}{}
	}
	// Close sync so that last invoke wont block on receive
	close(p.sync)
	return nil
}

// Take one thing off the unbuffered channel on each invocation, and send
// incremented counter to updates channel so that the test can assert monitor
// behaviour
func (p *fakeProcess) invoke(client *GithubClient) error {
	<-p.sync
	p.updates <- p.counter
	p.counter++
	return nil
}

// Test monitor can start a process obj and send it to task queue once a
// millisecond, and then return when the process obj exits
func TestMonitor(t *testing.T) {
	proc := &fakeProcess{
		sync:    make(chan struct{}),
		updates: make(chan int),
	}
	go monitor(&fakeTaskQueue{}, proc, time.Millisecond)
	assert.Equal(t, 0, <-proc.updates)
	assert.Equal(t, 1, <-proc.updates)
	assert.Equal(t, 2, <-proc.updates)
	assert.Equal(t, 3, <-proc.updates)
	assert.Equal(t, 4, <-proc.updates)
	// This message will only be sent once process exits
	assert.Equal(t, 5, <-proc.updates)
}
