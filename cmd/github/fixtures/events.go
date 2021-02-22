package fixtures

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const githubHeader = "X-Github-Event"

func GitHubPullRequestOpenedEvent(t *testing.T, headSHA, repo string) *http.Request {
	requestJSON, err := ioutil.ReadFile(filepath.Join("fixtures/githubPullRequestOpenedEvent.json"))
	require.NoError(t, err)

	// Replace sha with expected sha.
	requestJSONStr := strings.Replace(string(requestJSON), "f95f852bd8fca8fcc58a9a2d6c842781e32a215e", headSHA, -1)
	// Replace repo with expected repo.
	requestJSONStr = strings.Replace(string(requestJSON), "https://github.com/Codertocat/Hello-World.git", "file://"+repo, -1)

	req, err := http.NewRequest("POST", "/events", bytes.NewBuffer([]byte(requestJSONStr)))
	require.NoError(t, err)

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(githubHeader, "pull_request")

	return req
}
