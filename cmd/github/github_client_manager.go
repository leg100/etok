package github

import (
	"encoding/pem"
	"fmt"
	"io/ioutil"

	"k8s.io/klog/v2"
)

type githubClientManagerInterface interface {
	getOrCreate(int64) (*GithubClient, error)
}

// Cache of github clients keyed by install ID
type GithubClientManager struct {
	clients map[int64]*GithubClient

	hostname string
	keyPath  string
	appID    int64
}

func newGithubClientManager(hostname, keyPath string, appID int64) (*GithubClientManager, error) {
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

	return &GithubClientManager{
		appID:    appID,
		clients:  make(map[int64]*GithubClient),
		hostname: hostname,
		keyPath:  keyPath,
	}, nil
}

func (m *GithubClientManager) getOrCreate(installID int64) (*GithubClient, error) {
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
