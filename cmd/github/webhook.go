package github

import (
	"context"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"time"

	"github.com/google/go-github/v31/github"
	"github.com/gorilla/mux"
	"github.com/urfave/negroni"
	"k8s.io/klog/v2"
)

type githubApp interface {
	handleEvent(*GithubClient, interface{}) error
}

const githubHeader = "X-Github-Event"

type InstallationEvent interface {
	GetInstallation() *github.Installation
}

// WebhookServer listens for github events and dispatches them to a github app.
// Credentials for the app are required, which are used to create a github
// client. The client is provided to the app along with the event.
type webhookServer struct {
	// Github's hostname
	githubHostname string

	// Port on which to listen for github events
	port int

	// Webhook secret with which incoming events are validated - nil value skips
	// validation
	webhookSecret string

	appID   int64
	keyPath string

	// Maintain a github client per installation
	ghClientMgr GithubClientManager

	// Server context. Req handlers use the context to cancel background tasks.
	ctx context.Context

	// The github app to which to dispatch received events
	app githubApp
}

func newWebhookServer(app githubApp) *webhookServer {
	return &webhookServer{
		app:         app,
		ghClientMgr: newGithubClientManager(),
	}
}

func (o *webhookServer) validate() error {
	if o.webhookSecret == "" {
		return fmt.Errorf("webhook secret cannot be an empty string")
	}

	if o.appID == 0 {
		return fmt.Errorf("app-id cannot be zero")
	}
	klog.Infof("Github app ID: %d\n", o.appID)

	if o.keyPath == "" {
		return fmt.Errorf("key-path cannot be an empty string")
	}
	key, err := ioutil.ReadFile(o.keyPath)
	if err != nil {
		return fmt.Errorf("unable to read %s: %w", o.keyPath, err)
	}
	block, _ := pem.Decode(key)
	if block == nil || block.Type != "RSA PRIVATE KEY" {
		return fmt.Errorf("unable to decode private key in %s", o.keyPath)
	}

	return nil
}

func (o *webhookServer) run(ctx context.Context) error {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", o.port))
	if err != nil {
		return err
	}
	klog.Infof("Listening on %s\n", listener.Addr())

	r := mux.NewRouter()
	r.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Write(nil)
	})
	r.HandleFunc("/events", o.eventHandler).Methods("POST")

	// Add middleware
	n := negroni.New()
	n.Use(negroni.NewRecovery())
	n.Use(NewLogger())
	n.UseHandler(r)

	server := &http.Server{Handler: n}
	go func() {
		if err := server.Serve(listener); err != http.ErrServerClosed {
			klog.Error(err.Error())
		}
	}()

	<-ctx.Done()

	fmt.Println("Gracefully shutting down webhook server...")
	ctx, _ = context.WithTimeout(context.Background(), 5*time.Second) // nolint: vet
	return server.Shutdown(ctx)
}

func (o *webhookServer) eventHandler(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get(githubHeader) == "" {
		http.Error(w, "", http.StatusBadRequest)
		return
	}

	payload, err := github.ValidatePayload(r, []byte(o.webhookSecret))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	event, err := github.ParseWebHook(github.WebHookType(r), payload)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	install, ok := event.(InstallationEvent)
	if !ok || install.GetInstallation() == nil {
		// Irrelevant event
		klog.Infof("ignoring event: not associated with an app install")
		return
	}

	// Now we have an install ID we can fetch an install specific client
	client, err := o.ghClientMgr.getOrCreate(o.githubHostname, o.keyPath, o.appID, *install.GetInstallation().ID)
	if err != nil {
		panic(err.Error())
	}

	if err := o.app.handleEvent(client, event); err != nil {
		panic(err.Error())
	}

	w.Write(nil)
	return
}
