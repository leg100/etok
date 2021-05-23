package client

import (
	"context"
	"net/http"

	"github.com/bradleyfalzon/ghinstallation"
	"github.com/google/go-github/v31/github"
	"k8s.io/klog/v2"
)

// An invokable can be sent to the asynchronous client and will be called at
// some time in the future
type Invokable interface {
	Invoke(*github.Client) error
}

// An asynchronous wrapper around a github client (that authenticates as an
// installation). Adds the ability to asynchronously send requests to the GH
// API, as well synchronously.
type async struct {
	*github.Client
	transport *ghinstallation.Transport
	queue     chan Invokable
}

func newAsync(hostname string, appID, installID int64, key []byte) (*async, error) {
	transport, err := newTransport(hostname, appID, installID, key)
	if err != nil {
		return nil, err
	}

	client, err := newClient(hostname, &http.Client{Transport: transport})
	if err != nil {
		return nil, err
	}

	return &async{
		Client:    client,
		transport: transport,
		queue:     make(chan Invokable),
	}, nil
}

// Send a task to the asynchronous client to process at a later date
func (a *async) send(op Invokable) {
	a.queue <- op
}

// Process queue of operations against the GH API
func (a *async) process(ctx context.Context) {
	for {
		select {
		case op := <-a.queue:
			if err := op.Invoke(a.Client); err != nil {
				klog.Errorf("failed to invoke github API operation: %s", err.Error())
			}
		case <-ctx.Done():
			klog.Infof("github client: ending process queue: %s", ctx.Err().Error())
			return
		}
	}
}
