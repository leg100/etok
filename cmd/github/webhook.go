package github

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/google/go-github/v31/github"
	"github.com/gorilla/mux"
	"github.com/leg100/etok/pkg/client"
	"github.com/urfave/negroni"
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
	if !ok {
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
		if *ev.Action == "requested" || *ev.Action == "rerequested" {
			checkSuite := newWebhookCheckSuite(o.Client, ghClient, o.cloneDir, ev)
			if err := checkSuite.run(); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			w.Write([]byte("progressing..."))
			return
		}
	case *github.CheckRunEvent:
		opts := webhookCheckRunOptions{
		}

		switch *ev.Action {
		case "created":
			w.WriteHeader(http.StatusOK)
			return
		case "rerequested":
			checkRun, err := reRequestedCheckRun(ev.CheckRun, o.cloneDir, &opts)
			if err := (ev.CheckRun, o.cloneDir, &opts); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			w.Write([]byte("progressing..."))
			return
		}
	}

	// Irrelevant event
	w.WriteHeader(http.StatusOK)
	return
}
