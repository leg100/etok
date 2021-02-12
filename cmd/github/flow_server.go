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
	"time"

	assetfs "github.com/elazarl/go-bindata-assetfs"
	"github.com/gorilla/mux"
	"github.com/leg100/etok/pkg/static"
	"github.com/leg100/etok/pkg/vcs"
	"github.com/unrolled/render"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
)

const (
	// name of the secret containing the github app credentials
	secretName = "creds"
)

type errHttpServerInternalError struct{}

// flowServer handles the creation and setup of a new GitHub app
type flowServer struct {
	// Github's hostname
	githubHostname string

	// Optional github organization with which created app is to be associated,
	// (to be installed in?)
	githubOrg string

	// Name to be assigned to the github app
	name string

	// Listening port of flow server
	port int

	// Github webhook events are sent to this URL
	webhook string

	// HTML template renderer
	*render.Render

	// Toggle automatically opening flow server in browser
	disableBrowser bool

	listener net.Listener

	// Toggle development mode.
	devMode bool

	creds *credentials

	// Error channel for http handlers to report back a fatal error
	errch chan error

	success chan struct{}
}

func (s *flowServer) run(ctx context.Context) error {
	s.errch = make(chan error)

	// Validate and parse webhook url
	if s.webhook == "" {
		return fmt.Errorf("--webhook is required")
	}

	_, err := url.Parse(s.webhook)
	if err != nil {
		return err
	}

	if s.devMode {
		fmt.Println("Development mode is enabled")
	}

	// Listen on dynamic port (unless port set to non-zero)
	s.listener, err = net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", s.port))
	if err != nil {
		return err
	}

	// Setup template renderer
	s.Render = render.New(
		render.Options{
			Asset:         static.Asset,
			AssetNames:    static.AssetNames,
			Directory:     "static/templates",
			IsDevelopment: s.devMode,
		},
	)

	// Setup flow server routes
	r := mux.NewRouter()
	r.HandleFunc("/exchange-code", s.exchangeCode)
	r.HandleFunc("/github-app/setup", s.newApp)
	r.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Write(nil)
	})
	r.PathPrefix("/static/").Handler(http.FileServer(&assetfs.AssetFS{Asset: static.Asset, AssetDir: static.AssetDir, AssetInfo: static.AssetInfo}))

	http.Handle("/", r)

	// Run flow server
	server := &http.Server{}
	go func() {
		if err := server.Serve(s.listener); err != http.ErrServerClosed {
			panic(err.Error())
		}
	}()
	defer func() {
		fmt.Println("Gracefully shutting down web server...")
		server.Shutdown(ctx)
	}()

	// Wait for server to be running
	if err := s.wait(); err != nil {
		return err
	}

	if !s.disableBrowser {
		// Send user to browser to kick off app creation
		opener := getOpener()
		openArgs := append(opener, s.getUrl("/github-app/setup"))

		if err := exec.CommandContext(ctx, openArgs[0], openArgs[1:]...).Start(); err != nil {
			return err
		}
	}

	select {
	case <-s.success:
		return nil
	case err := <-s.errch:
		return err
	}
}

// Wait for web server to be running
func (s *flowServer) wait() error {
	if err := pollUrl(s.getUrl("/healthz"), 500*time.Millisecond, 10*time.Second); err != nil {
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

// Get a flow server URL with the given path
func (s *flowServer) getUrl(path string) string {
	return fmt.Sprintf("http://localhost:%d%s", s.getPort(), path)
}

// Get dynamically assigned port of flow server
func (s *flowServer) getPort() int {
	return s.listener.Addr().(*net.TCPAddr).Port
}

func (s *flowServer) githubNewAppUrl() string {
	u := &url.URL{
		Scheme: "https",
		Host:   s.githubHostname,
		Path:   "/settings/apps/new",
	}

	// https://developer.github.com/apps/building-github-apps/creating-github-apps-using-url-parameters/#about-github-app-url-parameters
	if s.githubOrg != "" {
		u.Path = fmt.Sprintf("organizations/%s%s", s.githubOrg, u.Path)
	}
	return u.String()
}

// newApp sends the user to github to create an app
func (s *flowServer) newApp(w http.ResponseWriter, r *http.Request) {
	manifest := &vcs.GithubManifest{
		Name:        s.name,
		Description: "etok",
		URL:         s.webhook,
		RedirectURL: s.getUrl("/exchange-code"),
		Public:      false,
		Webhook: &vcs.GithubWebhook{
			Active: true,
			URL:    fmt.Sprintf("%s/events", s.webhook),
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

	jsonManifest, err := json.MarshalIndent(manifest, "", " ")
	if err != nil {
		s.Render.Text(w, http.StatusBadRequest, "Failed to serialize manifest")
		s.errch <- err
		return
	}

	err = s.HTML(w, http.StatusOK, "github-app", struct {
		Target   string
		Manifest string
	}{
		Target:   s.githubNewAppUrl(),
		Manifest: string(jsonManifest),
	})
	if err != nil {
		s.errch <- err
		return
	}

	return
}

// exchangeCode handles the user coming back from creating their app. A code
// query parameter is exchanged for this app's ID, key, and webhook_secret
// Implements
// https://developer.github.com/apps/building-github-apps/creating-github-apps-from-a-manifest/#implementing-the-github-app-manifest-flow
func (s *flowServer) exchangeCode(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		s.Render.Text(w, http.StatusBadRequest, "Missing exchange code query parameter")
		s.errch <- errors.New("Missing exchange code query parameter")
		return
	}

	klog.V(1).Info("Exchanging GitHub app code for app credentials")

	creds := &vcs.GithubAnonymousCredentials{}
	client, err := vcs.NewGithubClient(s.githubHostname, creds)
	if err != nil {
		s.Render.Text(w, http.StatusInternalServerError, "Failed to instantiate github client")
		s.errch <- fmt.Errorf("Failed to instantiate github client: %w", err)
		return
	}

	app, err := client.ExchangeCode(code)
	if err != nil {
		s.Render.Text(w, http.StatusInternalServerError, "Failed to exchange code for github app")
		s.errch <- fmt.Errorf("Failed to exchange code for github app: %w", err)
		return
	}

	fmt.Printf("Found credentials for GitHub app %q with id %d\n", app.Name, app.ID)

	// Persist credentials to k8s secret
	if err := s.creds.create(context.Background(), app); err != nil {
		s.Render.Text(w, http.StatusInternalServerError, fmt.Sprintf("Unable to create secret %s: %s", s.creds, err.Error()))
		s.errch <- fmt.Errorf("Unable to create secret %s: %w", s.creds, err)
		return
	}
	fmt.Printf("Persisted credentials to secret %s\n", s.creds)

	http.Redirect(w, r, app.URL+"/installations/new", http.StatusFound)

	// Signal flow completion
	s.success <- struct{}{}
}
