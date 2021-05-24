package fixtures

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
var GithubConversionJSON = `{
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
