package fixtures

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dgrijalva/jwt-go"
	"github.com/gorilla/mux"
	"github.com/leg100/etok/pkg/github"
)

const GithubPrivateKey = `-----BEGIN RSA PRIVATE KEY-----
MIIEowIBAAKCAQEAuEPzOUE+kiEH1WLiMeBytTEF856j0hOVcSUSUkZxKvqczkWM
9vo1gDyC7ZXhdH9fKh32aapba3RSsp4ke+giSmYTk2mGR538ShSDxh0OgpJmjiKP
X0Bj4j5sFqfXuCtl9SkH4iueivv4R53ktqM+n6hk98l6hRwC39GVIblAh2lEM4L/
6WvYwuQXPMM5OG2Ryh2tDZ1WS5RKfgq+9ksNJ5Q9UtqtqHkO+E63N5OK9sbzpUUm
oNaOl3udTlZD3A8iqwMPVxH4SxgATBPAc+bmjk6BMJ0qIzDcVGTrqrzUiywCTLma
szdk8GjzXtPDmuBgNn+o6s02qVGpyydgEuqmTQIDAQABAoIBACL6AvkjQVVLn8kJ
dBYznJJ4M8ECo+YEgaFwgAHODT0zRQCCgzd+Vxl4YwHmKV2Lr+y2s0drZt8GvYva
KOK8NYYZyi15IlwFyRXmvvykF1UBpSXluYFDH7KaVroWMgRreHcIys5LqVSIb6Bo
gDmK0yBLPp8qR29s2b7ScZRtLaqGJiX+j55rNzrZwxHkxFHyG9OG+u9IsBElcKCP
kYCVE8ZdYexfnKOZbgn2kZB9qu0T/Mdvki8yk3I2bI6xYO24oQmhnT36qnqWoCBX
NuCNsBQgpYZeZET8mEAUmo9d+ABmIHIvSs005agK8xRaP4+6jYgy6WwoejJRF5yd
NBuF7aECgYEA50nZ4FiZYV0vcJDxFYeY3kYOvVuKn8OyW+2rg7JIQTremIjv8FkE
ZnwuF9ZRxgqLxUIfKKfzp/5l5LrycNoj2YKfHKnRejxRWXqG+ZETfxxlmlRns0QG
J4+BYL0CoanDSeA4fuyn4Bv7cy/03TDhfg/Uq0Aeg+hhcPE/vx3ebPsCgYEAy/Pv
eDLssOSdeyIxf0Brtocg6aPXIVaLdus+bXmLg77rJIFytAZmTTW8SkkSczWtucI3
FI1I6sei/8FdPzAl62/JDdlf7Wd9K7JIotY4TzT7Tm7QU7xpfLLYIP1bOFjN81rk
77oOD4LsXcosB/U6s1blPJMZ6AlO2EKs10UuR1cCgYBipzuJ2ADEaOz9RLWwi0AH
Pza2Sj+c2epQD9ZivD7Zo/Sid3ZwvGeGF13JyR7kLEdmAkgsHUdu1rI7mAolXMaB
1pdrsHureeLxGbRM6za3tzMXWv1Il7FQWoPC8ZwXvMOR1VQDv4nzq7vbbA8z8c+c
57+8tALQHOTDOgQIzwK61QKBgERGVc0EJy4Uag+VY8J4m1ZQKBluqo7TfP6DQ7O8
M5MX73maB/7yAX8pVO39RjrhJlYACRZNMbK+v/ckEQYdJSSKmGCVe0JrGYDuPtic
I9+IGfSorf7KHPoMmMN6bPYQ7Gjh7a++tgRFTMEc8956Hnt4xGahy9NcglNtBpVN
6G8jAoGBAMCh028pdzJa/xeBHLLaVB2sc0Fe7993WlsPmnVE779dAz7qMscOtXJK
fgtriltLSSD6rTA9hUAsL/X62rY0wdXuNdijjBb/qvrx7CAV6i37NK1CjABNjsfG
ZM372Ac6zc1EqSrid2IjET1YqyIW2KGLI1R2xbQc98UGlt48OdWu
-----END RSA PRIVATE KEY-----
`

// https://docs.github.com/en/rest/reference/apps#create-a-github-app-from-a-manifest
var githubConversionJSON = `{
	"id":      1,
	"node_id": "MDM6QXBwNTk=",
	"owner": {
		"login":               "octocat",
		"id":                  1,
		"node_id":             "MDQ6VXNlcjE=",
		"avatar_url":          "https://github.com/images/error/octocat_happy.gif",
		"gravatar_id":         "",
		"url":                 "https://api.github.com/users/octocat",
		"html_url":            "https://github.com/octocat",
		"followers_url":       "https://api.github.com/users/octocat/followers",
		"following_url":       "https://api.github.com/users/octocat/following{/other_user}",
		"gists_url":           "https://api.github.com/users/octocat/gists{/gist_id}",
		"starred_url":         "https://api.github.com/users/octocat/starred{/owner}{/repo}",
		"subscriptions_url":   "https://api.github.com/users/octocat/subscriptions",
		"organizations_url":   "https://api.github.com/users/octocat/orgs",
		"repos_url":           "https://api.github.com/users/octocat/repos",
		"events_url":          "https://api.github.com/users/octocat/events{/privacy}",
		"received_events_url": "https://api.github.com/users/octocat/received_events",
		"type":                "User",
		"site_admin":          false
	},
	"name":           "Etok",
	"description":    null,
	"external_url":   "https://etok.example.com",
	"html_url":       "https://%s/apps/etok",
	"created_at":     "2018-09-13T12:28:37Z",
	"updated_at":     "2018-09-13T12:28:37Z",
	"client_id":      "Iv1.8a61f9b3a7aba766",
	"client_secret":  "1726be1638095a19edd134c77bde3aa2ece1e5d8",
	"webhook_secret": "e340154128314309424b7c8e90325147d99fdafa",
	"pem":            "%s"
}`

var GithubAppInstallationJSON = `[
	{
		"id": 1,
		"account": {
			"login": "github",
			"id": 1,
			"node_id": "MDEyOk9yZ2FuaXphdGlvbjE=",
			"url": "https://api.github.com/orgs/github",
			"repos_url": "https://api.github.com/orgs/github/repos",
			"events_url": "https://api.github.com/orgs/github/events",
			"hooks_url": "https://api.github.com/orgs/github/hooks",
			"issues_url": "https://api.github.com/orgs/github/issues",
			"members_url": "https://api.github.com/orgs/github/members{/member}",
			"public_members_url": "https://api.github.com/orgs/github/public_members{/member}",
			"avatar_url": "https://github.com/images/error/octocat_happy.gif",
			"description": "A great organization"
		},
		"access_tokens_url": "https://api.github.com/installations/1/access_tokens",
		"repositories_url": "https://api.github.com/installation/repositories",
		"html_url": "https://github.com/organizations/github/settings/installations/1",
		"app_id": 1,
		"target_id": 1,
		"target_type": "Organization",
		"permissions": {
			"metadata": "read",
			"contents": "read",
			"issues": "write",
			"single_file": "write"
		},
		"events": [
			"push",
			"pull_request"
		],
		"single_file_name": "config.yml",
		"repository_selection": "selected"
	}
]`

// nolint: gosec
var GithubAppTokenJSON = `{
	"token":      "v1.1f699f1069f60xx%d",
	"expires_at": "2050-01-01T00:00:00Z",
	"permissions": {
		"issues":   "write",
		"contents": "read"
	},
	"repositories": [
		{
			"id":        1296269,
			"node_id":   "MDEwOlJlcG9zaXRvcnkxMjk2MjY5",
			"name":      "Hello-World",
			"full_name": "octocat/Hello-World",
			"owner": {
				"login":               "octocat",
				"id":                  1,
				"node_id":             "MDQ6VXNlcjE=",
				"avatar_url":          "https://github.com/images/error/octocat_happy.gif",
				"gravatar_id":         "",
				"url":                 "https://api.github.com/users/octocat",
				"html_url":            "https://github.com/octocat",
				"followers_url":       "https://api.github.com/users/octocat/followers",
				"following_url":       "https://api.github.com/users/octocat/following{/other_user}",
				"gists_url":           "https://api.github.com/users/octocat/gists{/gist_id}",
				"starred_url":         "https://api.github.com/users/octocat/starred{/owner}{/repo}",
				"subscriptions_url":   "https://api.github.com/users/octocat/subscriptions",
				"organizations_url":   "https://api.github.com/users/octocat/orgs",
				"repos_url":           "https://api.github.com/users/octocat/repos",
				"events_url":          "https://api.github.com/users/octocat/events{/privacy}",
				"received_events_url": "https://api.github.com/users/octocat/received_events",
				"type":                "User",
				"site_admin":          false
			},
			"private":           false,
			"html_url":          "https://github.com/octocat/Hello-World",
			"description":       "This your first repo!",
			"fork":              false,
			"url":               "https://api.github.com/repos/octocat/Hello-World",
			"archive_url":       "http://api.github.com/repos/octocat/Hello-World/{archive_format}{/ref}",
			"assignees_url":     "http://api.github.com/repos/octocat/Hello-World/assignees{/user}",
			"blobs_url":         "http://api.github.com/repos/octocat/Hello-World/git/blobs{/sha}",
			"branches_url":      "http://api.github.com/repos/octocat/Hello-World/branches{/branch}",
			"collaborators_url": "http://api.github.com/repos/octocat/Hello-World/collaborators{/collaborator}",
			"comments_url":      "http://api.github.com/repos/octocat/Hello-World/comments{/number}",
			"commits_url":       "http://api.github.com/repos/octocat/Hello-World/commits{/sha}",
			"compare_url":       "http://api.github.com/repos/octocat/Hello-World/compare/{base}...{head}",
			"contents_url":      "http://api.github.com/repos/octocat/Hello-World/contents/{+path}",
			"contributors_url":  "http://api.github.com/repos/octocat/Hello-World/contributors",
			"deployments_url":   "http://api.github.com/repos/octocat/Hello-World/deployments",
			"downloads_url":     "http://api.github.com/repos/octocat/Hello-World/downloads",
			"events_url":        "http://api.github.com/repos/octocat/Hello-World/events",
			"forks_url":         "http://api.github.com/repos/octocat/Hello-World/forks",
			"git_commits_url":   "http://api.github.com/repos/octocat/Hello-World/git/commits{/sha}",
			"git_refs_url":      "http://api.github.com/repos/octocat/Hello-World/git/refs{/sha}",
			"git_tags_url":      "http://api.github.com/repos/octocat/Hello-World/git/tags{/sha}",
			"git_url":           "git:github.com/octocat/Hello-World.git",
			"issue_comment_url": "http://api.github.com/repos/octocat/Hello-World/issues/comments{/number}",
			"issue_events_url":  "http://api.github.com/repos/octocat/Hello-World/issues/events{/number}",
			"issues_url":        "http://api.github.com/repos/octocat/Hello-World/issues{/number}",
			"keys_url":          "http://api.github.com/repos/octocat/Hello-World/keys{/key_id}",
			"labels_url":        "http://api.github.com/repos/octocat/Hello-World/labels{/name}",
			"languages_url":     "http://api.github.com/repos/octocat/Hello-World/languages",
			"merges_url":        "http://api.github.com/repos/octocat/Hello-World/merges",
			"milestones_url":    "http://api.github.com/repos/octocat/Hello-World/milestones{/number}",
			"notifications_url": "http://api.github.com/repos/octocat/Hello-World/notifications{?since,all,participating}",
			"pulls_url":         "http://api.github.com/repos/octocat/Hello-World/pulls{/number}",
			"releases_url":      "http://api.github.com/repos/octocat/Hello-World/releases{/id}",
			"ssh_url":           "git@github.com:octocat/Hello-World.git",
			"stargazers_url":    "http://api.github.com/repos/octocat/Hello-World/stargazers",
			"statuses_url":      "http://api.github.com/repos/octocat/Hello-World/statuses/{sha}",
			"subscribers_url":   "http://api.github.com/repos/octocat/Hello-World/subscribers",
			"subscription_url":  "http://api.github.com/repos/octocat/Hello-World/subscription",
			"tags_url":          "http://api.github.com/repos/octocat/Hello-World/tags",
			"teams_url":         "http://api.github.com/repos/octocat/Hello-World/teams",
			"trees_url":         "http://api.github.com/repos/octocat/Hello-World/git/trees{/sha}",
			"clone_url":         "https://github.com/octocat/Hello-World.git",
			"mirror_url":        "git:git.example.com/octocat/Hello-World",
			"hooks_url":         "http://api.github.com/repos/octocat/Hello-World/hooks",
			"svn_url":           "https://svn.github.com/octocat/Hello-World",
			"homepage":          "https://github.com",
			"language":          null,
			"forks_count":       9,
			"stargazers_count":  80,
			"watchers_count":    80,
			"size":              108,
			"default_branch":    "master",
			"open_issues_count": 0,
			"is_template":       true,
			"topics": [
				"octocat",
				"atom",
				"electron",
				"api"
			],
			"has_issues":    true,
			"has_projects":  true,
			"has_wiki":      true,
			"has_pages":     false,
			"has_downloads": true,
			"archived":      false,
			"disabled":      false,
			"visibility":    "public",
			"pushed_at":     "2011-01-26T19:06:43Z",
			"created_at":    "2011-01-26T19:01:12Z",
			"updated_at":    "2011-01-26T19:14:43Z",
			"permissions": {
				"admin": false,
				"push":  false,
				"pull":  true
			},
			"allow_rebase_merge":  true,
			"template_repository": null,
			"temp_clone_token":    "ABTLWHOULUVAXGTRYU7OC2876QJ2O",
			"allow_squash_merge":  true,
			"allow_merge_commit":  true,
			"subscribers_count":   42,
			"network_count":       0
		}
	]
}`

func ValidateGithubToken(tokenString string) error {
	key, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(GithubPrivateKey))
	if err != nil {
		return fmt.Errorf("could not parse private key: %s", err)
	}

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Don't forget to validate the alg is what you expect:
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			err := fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])

			return nil, err
		}

		return key.Public(), nil
	})

	if err != nil {
		return err
	}

	if claims, ok := token.Claims.(jwt.MapClaims); !ok || !token.Valid || claims["iss"] != "1" {
		return fmt.Errorf("Invalid token")
	}
	return nil
}

func GithubServerRouter(hostname string) http.Handler {
	counter := 0
	r := mux.NewRouter()
	r.HandleFunc("/settings/apps/new", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Unable to parse POST form"))
			return
		}

		manifest := github.GithubManifest{}
		manifestReader := strings.NewReader(r.PostFormValue("manifest"))
		if err := json.NewDecoder(manifestReader).Decode(&manifest); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Unable to decode JSON into a manifest object"))
			return
		}

		redirectURL := fmt.Sprintf("%s?code=good-code", manifest.RedirectURL)
		http.Redirect(w, r, redirectURL, http.StatusFound)
	})
	r.HandleFunc("/api/v3/app-manifests/good-code/conversions", func(w http.ResponseWriter, r *http.Request) {
		encodedKey := strings.Join(strings.Split(GithubPrivateKey, "\n"), "\\n")
		appInfo := fmt.Sprintf(githubConversionJSON, hostname, encodedKey)
		w.Write([]byte(appInfo)) // nolint: errcheck
	})
	r.HandleFunc("/apps/etok/installations/new", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("github app installation page")) // nolint: errcheck
	})
	r.HandleFunc("/api/v3/repos/Codertocat/Hello-World/statuses/changes", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})
	r.HandleFunc("/api/v3/app/installations", func(w http.ResponseWriter, r *http.Request) {
		token := strings.Replace(r.Header.Get("Authorization"), "Bearer ", "", 1)
		if err := ValidateGithubToken(token); err != nil {
			w.WriteHeader(403)
			w.Write([]byte("Invalid token")) // nolint: errcheck
			return
		}

		w.Write([]byte(GithubAppInstallationJSON)) // nolint: errcheck
	})
	r.HandleFunc("/api/v3/app/installations/123/access_tokens", func(w http.ResponseWriter, r *http.Request) {
		token := strings.Replace(r.Header.Get("Authorization"), "Bearer ", "", 1)
		if err := ValidateGithubToken(token); err != nil {
			w.WriteHeader(403)
			w.Write([]byte("Invalid token")) // nolint: errcheck
			return
		}

		appToken := fmt.Sprintf(GithubAppTokenJSON, counter)
		counter++
		w.Write([]byte(appToken)) // nolint: errcheck
	})
	return r
}

func GithubAppTestServer(t *testing.T) (string, error) {
	testServer := httptest.NewUnstartedServer(nil)

	// Our fake github router needs the hostname before starting server
	hostname := testServer.Listener.Addr().String()
	testServer.Config.Handler = GithubServerRouter(hostname)

	testServer.StartTLS()

	return hostname, nil
}
