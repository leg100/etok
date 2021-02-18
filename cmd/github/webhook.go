package github

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/go-github/v31/github"
	"github.com/gorilla/mux"
	"github.com/leg100/etok/pkg/util"
	"github.com/urfave/negroni"
	"k8s.io/klog/v2"
)

const githubHeader = "X-Github-Event"

type webhookServerOptions struct {
	// Github's hostname
	githubHostname string

	// Port on which to listen for github events
	port int

	creds *githubAppCredentials

	// Webhook secret with which incoming events are validated - nil value skips
	// validation
	webhookSecret []byte

	// Path to directory to which git repositories are cloned
	cloneDir string
}

func (o *webhookServerOptions) run(ctx context.Context) error {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", o.port))
	if err != nil {
		return err
	}

	r := mux.NewRouter()
	r.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Write(nil)
	})
	r.HandleFunc("/events", o.webhookEvent).Methods("POST")

	// Add middleware
	n := negroni.Classic()
	n.UseHandler(r)

	server := &http.Server{Handler: n}
	go func() {
		if err := server.Serve(listener); err == http.ErrServerClosed {
			klog.Error(err.Error())
		}
	}()

	<-ctx.Done()

	fmt.Println("Gracefully shutting down webhook server...")
	ctx, _ = context.WithTimeout(context.Background(), 5*time.Second) // nolint: vet
	return server.Shutdown(ctx)
}

func (o *webhookServerOptions) webhookEvent(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get(githubHeader) == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	payload, err := github.ValidatePayload(r, []byte(o.webhookSecret))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	_ = "X-Github-Delivery=" + r.Header.Get("X-Github-Delivery")
	event, _ := github.ParseWebHook(github.WebHookType(r), payload)
	pr, ok := event.(*github.PullRequestEvent)
	if !ok {
		// Ignoring unsupported event
		w.WriteHeader(http.StatusOK)
		return
	}

	if pr.Action == nil || (*pr.Action != "opened" && *pr.Action != "updated") {
		// Ignoring unsupported event
		w.WriteHeader(http.StatusOK)
		return
	}

	// Retrieve app install info from event
	install := pr.GetInstallation()
	if install == nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Github requires authentication against specific installation that
	// generated event
	o.creds.InstallationID = install.GetID()

	// Update status on PR
	client, err := newGithubClient(o.githubHostname, o.creds)
	if err != nil {
		// Ignoring unsupported event
		w.WriteHeader(http.StatusOK)
		return
	}

	updateStatus(client, "pending", "in progress...", "plan", pr)

	// Clone repo
	path := filepath.Join(o.cloneDir, strconv.FormatInt(*pr.PullRequest.ID, 10), util.GenerateRandomString(4))

	// Create the directory and parents if necessary.
	if err := os.MkdirAll(path, 0700); err != nil {
		panic(err.Error())
	}

	// Get fresh access token for cloning repo
	token, err := client.refreshToken()
	if err != nil {
		panic(err.Error())
	}

	// Insert access token into clone url
	authURL := fmt.Sprintf("://x-access-token:%s", token)
	url := strings.Replace(*pr.PullRequest.Head.Repo.CloneURL, "://:", authURL, 1)

	cmds := [][]string{
		{
			"git", "clone", "--branch", *pr.PullRequest.Head.Ref, "--depth=1", "--single-branch", url, path,
		},
	}

	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...) // nolint: gosec
		cmd.Dir = path

		_, err := cmd.CombinedOutput()
		if err != nil {
			panic(err.Error())
		}
	}
}
