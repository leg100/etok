package github

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/google/go-github/v31/github"
	"github.com/gorilla/mux"
	"github.com/leg100/etok/api/etok.dev/v1alpha1"
	"github.com/leg100/etok/pkg/client"
	"github.com/leg100/etok/pkg/launcher"
	"github.com/pkg/errors"
	"github.com/urfave/negroni"
	"k8s.io/klog/v2"
)

const githubHeader = "X-Github-Event"

type webhookServerOptions struct {
	// Github's hostname
	githubHostname string

	// Port on which to listen for github events
	port int

	creds *GithubAppCredentials

	// Webhook secret with which incoming events are validated - nil value skips
	// validation
	webhookSecret []byte

	// Path to directory to which git repositories are cloned
	cloneDir string

	*client.Client

	runName   string
	runStatus *v1alpha1.RunStatus
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

func validateAndParse(r *http.Request, webhookSecret []byte) (interface{}, error) {
	payload, err := github.ValidatePayload(r, webhookSecret)
	if err != nil {
		return nil, err
	}

	event, err := github.ParseWebHook(github.WebHookType(r), payload)
	if err != nil {
		return nil, err
	}

	return event, nil
}

func (o *webhookServerOptions) webhookEvent(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get(githubHeader) == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	event, err := validateAndParse(r, o.webhookSecret)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

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
		w.Write([]byte("no installation object found in payload"))
		return
	}

	// Github requires authentication against specific installation that
	// generated event
	o.creds.InstallationID = install.GetID()

	// Update status on PR
	client, err := NewGithubClient(o.githubHostname, o.creds)
	if err != nil {
		panic(err.Error())
	}

	if err := updateStatus(client, "pending", "in progress...", "init", pr); err != nil {
		panic(err.Error())
	}

	// Get fresh access token for cloning repo
	token, err := client.refreshToken()
	if err != nil {
		panic(err.Error())
	}

	path := filepath.Join(o.cloneDir, strconv.FormatInt(*pr.PullRequest.ID, 10))

	if err := o.cloneRepo(pr, token, path); err != nil {
		panic(err.Error())
	}

	w.Write([]byte("progressing..."))

	// Fire off commands
	go func() {
		workspaces, err := o.WorkspacesClient("").List(context.Background(), metav1.ListOptions{})
		if err != nil {
			panic(err.Error())
		}

		workspaces = filterWorkspaces(workspaces, pr)

		for _, ws := range workspaces.Items {
			if ws.Spec.VCS.Repository != *pr.Repo.CloneURL {
				// Ignore workspaces connected to a different repo
				continue
			}
			if _, err := os.Stat(filepath.Join(path, ws.Spec.VCS.WorkingDir)); err != nil {
				// Ignore workspaces connected to a non-existant sub-dir
				continue
			}

			// Construct launcher options
			launcherOpts := launcher.LauncherOptions{
				Client:     o.Client,
				Workspace:  ws.Name,
				Namespace:  ws.Namespace,
				DisableTTY: true,
				Command:    "plan",
				Path:       filepath.Join(path, ws.Spec.VCS.WorkingDir),
			}

			if o.runName != "" {
				launcherOpts.RunName = o.runName
			}

			if o.runStatus != nil {
				launcherOpts.Status = o.runStatus
			}

			// Launch init
			l, err := launcher.NewLauncher(&launcherOpts)
			if err != nil {
				panic(err.Error())
			}
			if err := l.Launch(context.Background()); err != nil {
				panic(err.Error())
			}
		}

		if err := updateStatus(client, "success", "successfully ran terraform plan", "plan", pr); err != nil {
			panic(err.Error())
		}
	}()
}

func filterWorkspaces(list *v1alpha1.WorkspaceList, pr *github.PullRequestEvent) *v1alpha1.WorkspaceList {
	newList := v1alpha1.WorkspaceList{}

	for _, ws := range list.Items {
		if ws.Spec.VCS.Repository != *pr.Repo.CloneURL {
			continue
		}
		newList.Items = append(newList.Items, ws)
	}

	return &newList
}

func (o *webhookServerOptions) cloneRepo(pr *github.PullRequestEvent, token, path string) error {
	// If the directory already exists, check if it's at the right commit.  If
	// so, then we do nothing.
	if _, err := os.Stat(path); err == nil {
		klog.V(1).Infof("clone directory %q already exists, checking if it's at the right commit", path)

		// We use git rev-parse to see if our repo is at the right commit.
		out, err := runGitCmd(path, "rev-parse", "HEAD")
		if err != nil {
			klog.Warningf("will re-clone repo, could not determine if was at correct commit: %s", err.Error())
			return o.forceClone(pr, token, path)
		}
		currCommit := strings.Trim(out, "\n")

		if currCommit == *pr.PullRequest.Head.SHA {
			// Do nothing: we're still at the current commit
			return nil
		}

		klog.V(1).Infof("repo was already cloned but is not at correct commit, wanted %q got %q", *pr.PullRequest.Head.SHA, currCommit)
		// We'll fall through to force-clone.
	}

	return o.forceClone(pr, token, path)
}

func (o *webhookServerOptions) forceClone(pr *github.PullRequestEvent, token, path string) error {
	// Insert access token into clone url
	u, err := url.Parse(*pr.PullRequest.Head.Repo.CloneURL)
	if err != nil {
		return fmt.Errorf("unable to parse clone url: %s: %w", *pr.PullRequest.Head.Repo.CloneURL, err)
	}
	u.User = url.UserPassword("x-access-token", token)

	// TODO: use url.Redacted to redact token in logging, errors

	if err := os.RemoveAll(path); err != nil {
		return errors.Wrapf(err, "deleting dir %q before cloning", path)
	}

	// Create the directory and parents if necessary.
	if err := os.MkdirAll(path, 0700); err != nil {
		return fmt.Errorf("unable to make directory for repo: %w", err)
	}

	args := []string{"clone", "--branch", *pr.PullRequest.Head.Ref, "--depth=1", "--single-branch", u.String(), path}
	if _, err := runGitCmd(path, args...); err != nil {
		return err
	}

	return nil
}

func runGitCmd(path string, args ...string) (string, error) {
	cmd := exec.Command("git", args...) // nolint: gosec
	cmd.Dir = path

	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("unable to run git command: %s: %w", args, err)
	}
	return string(out), nil
}
