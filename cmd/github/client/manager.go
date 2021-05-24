package client

import (
	"context"
	"fmt"
	"io/ioutil"
	"sync"

	"github.com/dgrijalva/jwt-go"
	"github.com/google/go-github/v31/github"
)

// Manager is a cache of github clients per install, permitting both synchronous
// and asynchronous requests
type Manager struct {
	clients map[int64]*async

	key   []byte
	appID int64

	mu sync.Mutex
}

func NewManager(keyPath string, appID int64) (*Manager, error) {
	if appID == 0 {
		return nil, fmt.Errorf("application ID cannot be zero")
	}

	key, err := ioutil.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("unable to read private key %s: %w", keyPath, err)
	}

	if err := validateKey(key); err != nil {
		return nil, err
	}

	return &Manager{
		appID:   appID,
		clients: make(map[int64]*async),
		key:     key,
	}, nil
}

// Token returns an access token for an installation with the given id and
// hostname
func (m *Manager) Token(ctx context.Context, id int64, hostname string) (string, error) {
	client, err := m.get(id, hostname)
	if err != nil {
		return "", err
	}
	return client.transport.Token(ctx)
}

// Send asynchronously sends a request to the Github API for a given
// installation.
func (m *Manager) Send(id int64, hostname string, op Invokable) error {
	client, err := m.get(id, hostname)
	if err != nil {
		return err
	}

	client.send(op)

	return nil
}

// Get retrieves a standard github client from the cache
func (m *Manager) Get(id int64, hostname string) (*github.Client, error) {
	client, err := m.get(id, hostname)
	if err != nil {
		return nil, err
	}
	return client.Client, nil
}

// get retrieves an asynchronous client from the cache
func (m *Manager) get(id int64, hostname string) (*async, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// See if we have a cached entry
	if c, ok := m.clients[id]; ok {
		return c, nil
	}

	// Create new cache entry, store, and return
	client, err := newAsync(hostname, m.appID, id, m.key)
	if err != nil {
		return nil, err
	}
	m.clients[id] = client
	return client, nil
}

func validateKey(key []byte) error {
	_, err := jwt.ParseRSAPrivateKeyFromPEM(key)
	if err != nil {
		return fmt.Errorf("unable to read private key: %w", err)
	}
	return nil
}
