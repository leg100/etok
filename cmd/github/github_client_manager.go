package github

// Cache of github clients keyed by install ID
type GithubClientManager map[int64]*GithubClient

func newGithubClientManager() GithubClientManager {
	return make(map[int64]*GithubClient)
}

func (m GithubClientManager) getOrCreate(hostname, keyPath string, appID, installID int64) (*GithubClient, error) {
	// See if we have a cached client
	ghClient, ok := m[installID]
	if !ok {
		// Create new client and cache it
		var err error
		ghClient, err = NewGithubAppClient(hostname, appID, keyPath, installID)
		if err != nil {
			return nil, err
		}
		m[installID] = ghClient
	}

	return ghClient, nil
}
