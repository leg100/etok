package github

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/google/go-github/v31/github"
	"github.com/gorilla/mux"
	"github.com/urfave/negroni"
	"k8s.io/klog/v2"
)

const githubHeader = "X-Github-Event"

// WebhookServer listens for github events and dispatches them to a github app.
// Credentials for the app are required, which are used to create a github
// client. The client is provided to the app along with the event.
type webhookServer struct {
	// Port on which to listen for github events
	port int

	// Webhook secret with which incoming events are validated - nil value skips
	// validation
	webhookSecret string

	// Server context. Req handlers use the context to cancel background tasks.
	ctx context.Context

	// The github app to which to dispatch received events
	app githubApp

	getter clientGetter
}

func (s *webhookServer) validate() error {
	if s.webhookSecret == "" {
		return fmt.Errorf("webhook secret cannot be an empty string")
	}

	return nil
}

func (s *webhookServer) run(ctx context.Context) error {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", s.port))
	if err != nil {
		return err
	}
	klog.Infof("Listening on %s\n", listener.Addr())

	// Record port for testing purposes (a test may want to know which port was
	// dynamically assigned)
	s.port = listener.Addr().(*net.TCPAddr).Port

	r := mux.NewRouter()
	r.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Write(nil)
	})
	r.HandleFunc("/events", s.eventHandler).Methods("POST")

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

	klog.V(1).Info("Shutting down webhook server")
	ctx, _ = context.WithTimeout(context.Background(), 5*time.Second) // nolint: vet
	return server.Shutdown(ctx)
}

func (s *webhookServer) eventHandler(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get(githubHeader) == "" {
		http.Error(w, "", http.StatusBadRequest)
		return
	}

	payload, err := github.ValidatePayload(r, []byte(s.webhookSecret))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	event, err := github.ParseWebHook(github.WebHookType(r), payload)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ievent, ok := event.(installEvent)
	if !ok {
		http.Error(w, "github app installation ID not found in event", http.StatusBadRequest)
		return
	}

	// Retrieve github client for install
	client, err := s.getter.Get(ievent.GetInstallation().GetID(), "github.com")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := s.app.handleEvent(event, client.Checks); err != nil {
		panic(err.Error())
	}

	w.Write(nil)
	return
}
