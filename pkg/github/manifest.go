package github

// appRequest contains the query parameters for
// https://developer.github.com/apps/building-github-apps/creating-github-apps-from-a-manifest
type GithubManifest struct {
	Description string            `json:"description"`
	Events      []string          `json:"default_events"`
	Name        string            `json:"name"`
	Permissions map[string]string `json:"default_permissions"`
	Public      bool              `json:"public"`
	RedirectURL string            `json:"redirect_url"`
	SetupURL    string            `json:"setup_url"`
	URL         string            `json:"url"`
	Webhook     *GithubWebhook    `json:"hook_attributes"`
}

type GithubWebhook struct {
	Active bool   `json:"active"`
	URL    string `json:"url"`
}
