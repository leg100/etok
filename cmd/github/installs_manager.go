package github

import (
	"encoding/pem"
	"fmt"
	"io/ioutil"

	"k8s.io/klog/v2"
)

// Github manager manages access to installs of the github app (on github.com,
// github enterprise, etc)
type installsManager interface {
	tokenRefresher
	sender
}

// A tokenRefresher can provide a fresh token for authenticating git operations
// with github for a given github app install ID.
type tokenRefresher interface {
	refreshToken(int64) (string, error)
}

// Send an invoker to a client whith which it'll be invoked
type sender interface {
	send(int64, invoker) error
}

// An invoker is capable of performing some action against the github API
type invoker interface {
	invoke(*GithubClient) error
}

// Implementation of installsManager
type installsManagerImpl struct {
	clients map[int64]*GithubClient

	hostname string
	keyPath  string
	appID    int64
}

func newInstallsManager(hostname, keyPath string, appID int64) (*installsManagerImpl, error) {
	if appID == 0 {
		return nil, fmt.Errorf("app-id cannot be zero")
	}
	klog.Infof("Github app ID: %d\n", appID)

	// Validate private key
	if keyPath == "" {
		return nil, fmt.Errorf("key-path cannot be an empty string")
	}
	key, err := ioutil.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("unable to read %s: %w", keyPath, err)
	}
	block, _ := pem.Decode(key)
	if block == nil || block.Type != "RSA PRIVATE KEY" {
		return nil, fmt.Errorf("unable to decode private key in %s", keyPath)
	}

	return &installsManagerImpl{
		appID:    appID,
		clients:  make(map[int64]*GithubClient),
		hostname: hostname,
		keyPath:  keyPath,
	}, nil
}

// Get a github client for the install ID from the cache, or if not found,
// create new client.
func (m *installsManagerImpl) getOrCreate(installID int64) (*GithubClient, error) {
	// See if we have a cached client
	ghClient, ok := m.clients[installID]
	if !ok {
		// Create new client and cache it
		var err error
		ghClient, err = NewGithubAppClient(m.hostname, m.appID, m.keyPath, installID)
		if err != nil {
			return nil, err
		}
		m.clients[installID] = ghClient
	}

	return ghClient, nil
}

func (m *installsManagerImpl) refreshToken(installID int64) (string, error) {
	client, err := m.getOrCreate(installID)
	if err != nil {
		return "", err
	}
	return client.refreshToken()
}

func (m *installsManagerImpl) send(installID int64, inv invoker) error {
	client, err := m.getOrCreate(installID)
	if err != nil {
		return err
	}
	client.send(inv)

	return nil
}
