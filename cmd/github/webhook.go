package github

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/go-github/v31/github"
	"github.com/gorilla/mux"
	"github.com/leg100/etok/pkg/client"
	"github.com/urfave/negroni"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

const githubHeader = "X-Github-Event"

type InstallationEvent interface {
	GetInstallation() *github.Installation
}

type webhookServer struct {
	// Github's hostname
	githubHostname string

	// Port on which to listen for github events
	port int

	// Webhook secret with which incoming events are validated - nil value skips
	// validation
	webhookSecret []byte

	// Path to directory to which git repositories are cloned
	cloneDir string

	*client.Client

	appID   int64
	keyPath string

	// Maintain a github client per installation
	ghClientMap GithubClientMap

	// Permit tests to override run status
	checkRunOptions
}

func newWebhookServer() *webhookServer {
	return &webhookServer{
		ghClientMap: newGithubClientMap(),
	}
}

func (o *webhookServer) run(ctx context.Context) error {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", o.port))
	if err != nil {
		return err
	}

	r := mux.NewRouter()
	r.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Write(nil)
	})
	r.HandleFunc("/events", o.eventHandler).Methods("POST")

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

func (o *webhookServer) eventHandler(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get(githubHeader) == "" {
		http.Error(w, "", http.StatusBadRequest)
		return
	}

	event, err := validateAndParse(r, o.webhookSecret)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	install, ok := event.(InstallationEvent)
	if !ok || install.GetInstallation() == nil {
		// Irrelevant event
		w.WriteHeader(http.StatusOK)
		return
	}

	// Get github client now we have install ID
	ghClient, err := o.ghClientMap.getClient(o.githubHostname, o.keyPath, o.appID, *install.GetInstallation().ID)
	if err != nil {
		panic(err.Error())
	}

	switch ev := event.(type) {
	case *github.CheckSuiteEvent:
		switch *ev.Action {
		case "requested", "rerequested":
			// Get access token for cloning repo
			token, err := ghClient.refreshToken()
			if err != nil {
				panic(err.Error())
			}

			// Ensure we have a cloned repo on disk
			err = ensureCloned(*ev.Repo.CloneURL, clonePath(o.cloneDir, *ev.CheckSuite.ID), *ev.CheckSuite.HeadBranch, *ev.CheckSuite.HeadSHA, token)
			if err != nil {
				panic(err.Error())
			}

			// Find connected workspaces and create a check-run for each one.
			// Record the workspace name in the check-run's ExternalID attribute
			// so that if the check-run needs to be re-run we know which
			// workspace to use.
			connected, err := getConnectedWorkspaces(o.Client, *ev.Repo.CloneURL)
			if err != nil {
				panic(err.Error())
			}

			// Create check-run for each connected workspace
			for _, ws := range connected.Items {
				// Get full path to workspace's working directory
				path := workspacePath(o.cloneDir, *ev.CheckSuite.ID, ws.Spec.VCS.WorkingDir)

				// Skip workspaces with a non-existent working dir
				if _, err := os.Stat(path); os.IsNotExist(err) {
					klog.Warningf("skipping workspace %s with non-existent working directory: %s", klog.KObj(&ws), ws.Spec.VCS.WorkingDir)
					continue
				}

				// TODO: github API calls will eventually use a transport that
				// deliberately delays calls in order to avoid rate-limiting.
				// Once we do that we might want to consider employing a channel
				// instead.
				name := klog.KObj(&ws).String()
				checkRun, _, err := ghClient.Checks.CreateCheckRun(context.Background(), *ev.Repo.Owner.Login, *ev.Repo.Name, github.CreateCheckRunOptions{
					Name:       name,
					HeadSHA:    *ev.CheckSuite.HeadSHA,
					ExternalID: &name,
					Status:     github.String("inprogress"),
				})
				if err != nil {
					klog.Errorf("unable to create check run : %s", err.Error())
					continue
				}

				// Launch an etok run in a goroutine
				go launch(checkRun, ghClient, o.Client, path, ws.Name, ws.Namespace, &o.checkRunOptions)
			}

			w.Write([]byte("check runs created"))
		}
	case *github.CheckRunEvent:
		switch *ev.Action {
		case "rerequested":
			// User has requested that a check-run be re-created. We recorded
			// the connected Workspace in the original check-run's ExternalID
			// attribute, so we can use that for the new check-run.
			parts := strings.Split(*ev.CheckRun.ExternalID, "/")
			if len(parts) != 2 {
				panic("found malformed check-run external ID: " + *ev.CheckRun.ExternalID)
			}
			namespace, workspace := parts[0], parts[1]

			ws, err := o.WorkspacesClient(namespace).Get(context.Background(), workspace, metav1.GetOptions{})
			if err != nil {
				panic(err.Error())
			}

			// Get full path to workspace's working directory
			path := workspacePath(o.cloneDir, *ev.CheckRun.CheckSuite.ID, ws.Spec.VCS.WorkingDir)

			// Get access token for cloning repo
			token, err := ghClient.refreshToken()
			if err != nil {
				panic(err.Error())
			}

			// Ensure we have a cloned repo on disk
			err = ensureCloned(*ev.Repo.CloneURL, clonePath(o.cloneDir, *ev.CheckRun.CheckSuite.ID), *ev.CheckRun.CheckSuite.HeadBranch, *ev.CheckRun.HeadSHA, token)
			if err != nil {
				panic(err.Error())
			}

			checkRun, _, err := ghClient.Checks.CreateCheckRun(context.Background(), *ev.Repo.Owner.Login, *ev.Repo.Name, github.CreateCheckRunOptions{
				Name:       *ev.CheckRun.ExternalID,
				HeadSHA:    *ev.CheckRun.HeadSHA,
				ExternalID: ev.CheckRun.ExternalID,
			})
			if err != nil {
				panic(err.Error())
			}

			// Launch an etok run in a goroutine
			go launch(checkRun, ghClient, o.Client, path, ws.Name, ws.Namespace, &checkRunOptions{})

			w.Write([]byte("re-running check run"))
		}
	}

	w.WriteHeader(http.StatusOK)
	return
}
