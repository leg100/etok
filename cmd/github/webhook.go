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

	"github.com/google/go-github/v31/github"
	"github.com/gorilla/mux"
	"github.com/leg100/etok/pkg/util"
	"github.com/leg100/etok/pkg/vcs"
)

const githubHeader = "X-Github-Event"

type webhookServerOptions struct {
	// Port on which to listen for github events
	port int

	creds *vcs.GithubAppCredentials

	webhookSecret string

	// Path to directory to which git repositories are cloned
	cloneDir string
}

func (o *webhookServerOptions) run(ctx context.Context) error {
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", o.port))
	if err != nil {
		return err
	}

	r := mux.NewRouter()
	r.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Write(nil)
	})
	r.HandleFunc("/events", o.webhookEvent).Methods("POST")

	server := &http.Server{Handler: r}
	go func() {
		if err := server.Serve(listener); err != http.ErrServerClosed {
			panic(err.Error())
		}
	}()
	defer func() {
		fmt.Println("Gracefully shutting down webhook server...")
		server.Shutdown(ctx)
	}()

	return nil
}

func (o *webhookServerOptions) webhookEvent(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get(githubHeader) == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	payload, err := github.ValidatePayload(r, []byte(o.webhookSecret))
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
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

	install := pr.GetInstallation()
	if install == nil {
		// Ignoring unsupported event
		w.WriteHeader(http.StatusOK)
		return
	}

	install.CreatedAt

	// Github requires authentication against installation that generated event
	o.creds.InstallationID = install.GetID()

	// Update status on PR
	client, err := vcs.NewGithubClient(o.creds.Hostname, o.creds)
	if err != nil {
		// Ignoring unsupported event
		w.WriteHeader(http.StatusOK)
		return
	}

	vcs.UpdateStatus(client, "pending", "in progress...", "plan", pr)

	// Clone repo
	path := filepath.Join(o.cloneDir, strconv.FormatInt(*pr.PullRequest.ID, 10), util.GenerateRandomString(4))

	// Create the directory and parents if necessary.
	if err := os.MkdirAll(path, 0700); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	token, err := o.creds.GetToken()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
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
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
}
