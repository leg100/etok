package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"time"

	assetfs "github.com/elazarl/go-bindata-assetfs"
	"github.com/gorilla/mux"
	"github.com/leg100/etok/pkg/github"
	"github.com/leg100/etok/pkg/static"
	"github.com/unrolled/render"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
)

type errHttpServerInternalError struct{}

// appCreator handles the creation and setup of a new GitHub app
type appCreator struct {
	*render.Render

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

	creator := appCreator{
		creds: creds,
		errch: make(chan error),
		port:  listener.Addr().(*net.TCPAddr).Port,
		Render: render.New(
			render.Options{
				Asset:         static.Asset,
				AssetNames:    static.AssetNames,
				Directory:     "static/templates",
				IsDevelopment: opts.devMode,
			},
		),
		githubHostname: githubHostname,
		githubOrg:      opts.githubOrg,
	}

	// Serialize manifest as JSON ready to be POST'd
	creator.manifestJson, err = manifestJson(appName, webhookUrl, creator.getUrl("/exchange-code"))
	if err != nil {
		return fmt.Errorf("unable to serialize manifest to JSON: %w", err)
	}

	// Setup manifest server routes
	r := mux.NewRouter()
	r.HandleFunc("/exchange-code", creator.exchangeCode)
	r.HandleFunc("/github-app/setup", creator.newApp)
	r.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) { w.Write(nil) })
	r.PathPrefix("/static/").Handler(http.FileServer(&assetfs.AssetFS{Asset: static.Asset, AssetDir: static.AssetDir, AssetInfo: static.AssetInfo}))

	if !opts.disableBrowser {
		// Send user to browser to kick off app creation
		if err := open(ctx, creator.getUrl("/github-app/setup")); err != nil {
			return err
		}
	}

	server := &http.Server{Handler: r}
	go func() {
		if err := server.Serve(listener); err != http.ErrServerClosed {
			creator.errch <- fmt.Errorf("unable to start web server: %w", err)
		}
	}()
	defer func() {
		fmt.Println("Gracefully shutting down web server...")
		server.Shutdown(ctx)
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-creator.errch:
		return err
	}
}

// Wait for web server to be running
func (c *appCreator) wait() error {
	if err := pollUrl(c.getUrl("/healthz"), 500*time.Millisecond, 10*time.Second); err != nil {
		return fmt.Errorf("encountered error while waiting for web server to startup: %w", err)
	}

	return nil
}

// pollUrl polls a url every interval until timeout. If an HTTP 200 is received
// it exits without error.
func pollUrl(url string, interval, timeout time.Duration) error {
	return wait.PollImmediate(interval, timeout, func() (bool, error) {
		resp, err := http.Get(url)
		if err != nil {
			klog.V(2).Infof("polling %s: %s", url, err.Error())
			return false, nil
		}
		if resp.StatusCode == 200 {
			return true, nil
		}
		return false, nil
	})
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
	err := c.HTML(w, http.StatusOK, "github-app", struct {
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
		c.Render.Text(w, http.StatusBadRequest, "Missing exchange code query parameter")
		c.errch <- errors.New("Missing exchange code query parameter")
		return
	}

	klog.V(1).Info("Exchanging GitHub app code for app credentials")

	client, err := NewAnonymousGithubClient(c.githubHostname)
	if err != nil {
		c.Render.Text(w, http.StatusInternalServerError, "Failed to instantiate github client")
		c.errch <- fmt.Errorf("Failed to instantiate github client: %w", err)
		return
	}

	ctx := context.Background()
	cfg, _, err := client.Apps.CompleteAppManifest(ctx, code)
	if err != nil {
		c.Render.Text(w, http.StatusInternalServerError, "Failed to exchange code for github app")
		c.errch <- fmt.Errorf("Failed to exchange code for github app: %w", err)
		return
	}

	fmt.Printf("Found credentials for GitHub app %q with id %d\n", cfg.GetName(), cfg.GetID())

	// Persist credentials to k8s secret
	if err := c.creds.create(context.Background(), cfg); err != nil {
		c.Render.Text(w, http.StatusInternalServerError, fmt.Sprintf("Unable to create secret %s: %s", c.creds, err.Error()))
		c.errch <- fmt.Errorf("Unable to create secret %s: %w", c.creds, err)
		return
	}
	fmt.Printf("Persisted credentials to secret %s\n", c.creds)

	http.Redirect(w, r, cfg.GetHTMLURL()+"/installations/new", http.StatusFound)

	// Signal manifest flow completion
	c.errch <- nil
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

func manifestJson(appName, webhookUrl, redirectUrl string) (string, error) {
	m := &github.GithubManifest{
		Name:        appName,
		Description: "etok",
		URL:         webhookUrl,
		RedirectURL: redirectUrl,
		Public:      false,
		Webhook: &github.GithubWebhook{
			Active: true,
			URL:    fmt.Sprintf("%s/events", webhookUrl),
		},
		Events: []string{
			"check_run",
			"create",
			"delete",
			"issue_comment",
			"issues",
			"pull_request_review_comment",
			"pull_request_review",
			"pull_request",
			"push",
		},
		Permissions: map[string]string{
			"checks":           "write",
			"contents":         "write",
			"issues":           "write",
			"pull_requests":    "write",
			"repository_hooks": "write",
			"statuses":         "write",
		},
	}

	marshalled, err := json.MarshalIndent(m, "", " ")
	return string(marshalled), err
}
