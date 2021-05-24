package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"time"

	"github.com/gorilla/mux"
	"github.com/leg100/etok/cmd/github/client"
	"github.com/leg100/etok/pkg/github"
	"k8s.io/klog/v2"
)

// appCreator handles the creation and setup of a new GitHub app
type appCreator struct {
	// Error channel for http handlers to report back a fatal error
	errch chan error

	// Started channel receives empty struct when server has successfully
	// started up
	started chan struct{}

	// Listening port
	port int

	// Github's hostname
	githubHostname string

	// Optional github organization with which created app is to be associated,
	// (to be installed in?)
	githubOrg string

	creds *credentials

	manifestJson string

	// HTML template for rendering web pages
	tmpl *template.Template
}

type createAppOptions struct {
	// Toggle automatically opening manifest server in browser
	disableBrowser bool

	// Toggle development mode.
	devMode bool

	// Set to non-zero to override default behaviour of dynamically assigning
	// listening port
	port int

	// Optional github organization with which created app is to be associated,
	// (to be installed in?)
	githubOrg string
}

func createApp(ctx context.Context, appName, webhookUrl, githubHostname string, creds *credentials, opts createAppOptions) error {
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", opts.port))
	if err != nil {
		return err
	}

	tmpl, err := template.ParseFS(static, "static/templates/github-app.tmpl")
	if err != nil {
		return err
	}

	creator := appCreator{
		creds:          creds,
		errch:          make(chan error),
		port:           listener.Addr().(*net.TCPAddr).Port,
		githubHostname: githubHostname,
		githubOrg:      opts.githubOrg,
		tmpl:           tmpl,
	}

	// Serialize manifest as JSON ready to be POST'd
	creator.manifestJson, err = manifestJson(appName, webhookUrl, creator.getUrl("/exchange-code"), creator.getUrl("/github-app/installed"))
	if err != nil {
		return fmt.Errorf("unable to serialize manifest to JSON: %w", err)
	}

	// Setup manifest server routes
	r := mux.NewRouter()
	r.HandleFunc("/exchange-code", creator.exchangeCode)
	r.HandleFunc("/github-app/setup", creator.newApp)
	r.HandleFunc("/github-app/installed", creator.installed)
	r.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) { w.Write(nil) })
	r.PathPrefix("/static/").Handler(http.FileServer(http.FS(static)))

	if !opts.disableBrowser {
		// Send user to browser to kick off app creation
		url := creator.getUrl("/github-app/setup")
		if err := open(ctx, url); err != nil {
			return err
		}
		fmt.Printf("Your browser has been opened to visit: %s\n", url)
	}

	server := &http.Server{Handler: r}
	go func() {
		if err := server.Serve(listener); err != http.ErrServerClosed {
			creator.errch <- fmt.Errorf("unable to start web server: %w", err)
		}
	}()
	defer server.Shutdown(ctx)

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-creator.errch:
		// Give user's browser an opportunity to download any assets before
		// shutting down web server.
		time.Sleep(time.Second)

		return err
	}
}

// Get a manifest server URL with the given path
func (c *appCreator) getUrl(path string) string {
	return fmt.Sprintf("http://localhost:%d%s", c.port, path)
}

func (c *appCreator) githubNewAppUrl() string {
	u := &url.URL{
		Scheme: "https",
		Host:   c.githubHostname,
		Path:   "/settings/apps/new",
	}

	// https://developer.github.com/apps/building-github-apps/creating-github-apps-using-url-parameters/#about-github-app-url-parameters
	if c.githubOrg != "" {
		u.Path = fmt.Sprintf("organizations/%s%s", c.githubOrg, u.Path)
	}
	return u.String()
}

// newApp sends the user to github to create an app
func (c *appCreator) newApp(w http.ResponseWriter, r *http.Request) {
	err := c.tmpl.Execute(w, struct {
		Target   string
		Manifest string
	}{
		Target:   c.githubNewAppUrl(),
		Manifest: c.manifestJson,
	})
	if err != nil {
		c.errch <- err
		return
	}

	return
}

// exchangeCode handles the user coming back from creating their app. A code
// query parameter is exchanged for this app's ID, key, and webhook_secret
// Implements
// https://developer.github.com/apps/building-github-apps/creating-github-apps-from-a-manifest/#implementing-the-github-app-manifest-flow
func (c *appCreator) exchangeCode(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		w.WriteHeader(http.StatusBadRequest)
		c.errch <- errors.New("Missing exchange code query parameter")
		return
	}

	klog.V(1).Info("Exchanging GitHub app code for app credentials")

	client, err := client.NewAnonymous(c.githubHostname)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		c.errch <- fmt.Errorf("Failed to instantiate github client: %w", err)
		return
	}

	cfg, _, err := client.Apps.CompleteAppManifest(context.Background(), code)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		c.errch <- fmt.Errorf("Failed to exchange code for github app: %w", err)
		return
	}

	fmt.Printf("Successfully created github app %q. App ID: %d\n", cfg.GetName(), cfg.GetID())

	// Persist credentials to k8s secret
	if err := c.creds.create(context.Background(), cfg); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		c.errch <- fmt.Errorf("Unable to create secret %s: %w", c.creds, err)
		return
	}
	fmt.Printf("Persisted credentials to secret %s\n", c.creds)

	http.Redirect(w, r, cfg.GetHTMLURL()+"/installations/new", http.StatusFound)
}

// installed is a web page providing confirmation to the user that the app has
// installed successfully
func (c *appCreator) installed(w http.ResponseWriter, r *http.Request) {
	err := c.tmpl.Execute(w, nil)
	if err != nil {
		c.errch <- fmt.Errorf("unable to render confirmation page: %w", err)
		return
	}

	fmt.Printf("Github app successfully installed. Installation ID: %s\n", r.URL.Query().Get("installation_id"))

	// Signal manifest flow completion
	c.errch <- nil
	return
}

func open(ctx context.Context, args ...string) error {
	args = append(getOpener(), args...)
	return exec.CommandContext(ctx, args[0], args[1:]...).Start()
}

func getOpener() []string {
	switch runtime.GOOS {
	case "windows":
		return []string{"cmd", "/c", "start"}
	case "darwin":
		return []string{"open"}
	default: // "linux", "freebsd", "openbsd", "netbsd"
		return []string{"xdg-open"}
	}
}

func manifestJson(appName, webhookUrl, redirectUrl, setupUrl string) (string, error) {
	// appRequest contains the query parameters for
	// https://developer.github.com/apps/building-github-apps/creating-github-apps-from-a-manifest
	m := &github.GithubManifest{
		Name:        appName,
		Description: "etok",
		URL:         webhookUrl,
		SetupURL:    setupUrl,
		RedirectURL: redirectUrl,
		Public:      false,
		Webhook: &github.GithubWebhook{
			Active: true,
			URL:    fmt.Sprintf("%s/events", webhookUrl),
		},
		Events: []string{
			"check_run",
			"check_suite",
			"pull_request_review_comment",
			"pull_request_review",
			"pull_request",
		},
		Permissions: map[string]string{
			"checks":        "write",
			"contents":      "read",
			"pull_requests": "read",
		},
	}

	marshalled, err := json.MarshalIndent(m, "", " ")
	return string(marshalled), err
}
