package fixtures

import (
	"fmt"

	"github.com/dgrijalva/jwt-go"
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

// nolint: gosec
const GithubAppTokenJSON = `{
	"token":      "v1.1f699f1069f60xxx",
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
