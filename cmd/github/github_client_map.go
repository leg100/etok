package github

// Cache of github clients keyed by install ID
type GithubClientMap map[int64]*GithubClient

func (m GithubClientMap) getClient(hostname, keyPath string, appID, installID int64) (*GithubClient, error) {
	if m == nil {
		m = make(map[int64]*GithubClient)
	}

	// See if we have a cached client
	ghClient, ok := m[installID]
	if !ok {
		// Create new client and cache it
		ghClient, err := NewGithubAppClient(hostname, appID, keyPath, installID)
		if err != nil {
			return nil, err
		}
		m[installID] = ghClient
	}

	return ghClient, nil
}
