package vcs

import (
	"net/http"
)

// GithubCredentials handles creating http.Clients that authenticate.
type GithubCredentials interface {
	Client() (*http.Client, error)
	GetToken() (string, error)
	GetUser() string
}
