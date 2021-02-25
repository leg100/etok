package fixtures

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	"github.com/google/go-github/v31/github"
	"github.com/stretchr/testify/require"
)

const githubHeader = "X-Github-Event"

func GitHubNewCheckSuiteEvent(t *testing.T, headSHA, repo string) *http.Request {
	eventStr := newCheckSuiteEvent(headSHA, repo)

	req, err := http.NewRequest("POST", "/events", bytes.NewBuffer([]byte(eventStr)))
	require.NoError(t, err)

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(githubHeader, "check_suite")

	return req
}

func GithubCheckSuite(headSHA, repo string) *github.CheckSuite {
	eventStr := newCheckSuiteEvent(headSHA, repo)

	var event github.CheckSuiteEvent

	_ = json.Unmarshal([]byte(eventStr), &event)

	// Code under test expects the check suite's repository to be populated
	event.CheckSuite.Repository = event.Repo

	return event.CheckSuite
}

func newCheckSuiteEvent(headSHA, repo string) string {
	requestJSON, _ := ioutil.ReadFile("fixtures/newCheckSuiteEvent.json")

	// Replace sha with expected sha.
	requestJSONStr := strings.Replace(string(requestJSON), "ec26c3e57ca3a959ca5aad62de7213c562f8c821", headSHA, -1)
	// Replace repo with expected repo.
	requestJSONStr = strings.Replace(string(requestJSON), "https://github.com/Codertocat/Hello-World.git", "file://"+repo, -1)

	return requestJSONStr
}
