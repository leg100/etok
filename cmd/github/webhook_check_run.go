package github

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/google/go-github/v31/github"
	"github.com/leg100/etok/api/etok.dev/v1alpha1"
	cmdutil "github.com/leg100/etok/cmd/util"
	"github.com/leg100/etok/pkg/client"
	"github.com/leg100/etok/pkg/launcher"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

const (
	checkRunLabel = "etok.dev/github-check-run-id"
)

type webhookCheckRunOptions struct {
	ws       *v1alpha1.Workspace
	ghClient *GithubClient
	*client.Client
	runName   string
	runStatus *v1alpha1.RunStatus
}

type webhookCheckRun struct {
	*github.CheckRun

	*webhookCheckRunOptions
	login string
	repo  string
	path  string
	out   *bytes.Buffer

	namespace string
	workspace string
}

func validateCheckRunOptions(opts *webhookCheckRunOptions) error {
	if opts.ghClient == nil {
		return errors.New("github client cannot be nil")
	}
	if opts.Client == nil {
		return errors.New("k8s client cannot be nil")
	}
	return nil
}

func newWebhookCheckRun(checkRun *github.CheckRun, login, repo, path string, ws *v1alpha1.Workspace, opts *webhookCheckRunOptions) (*webhookCheckRun, error) {
	if err := validateCheckRunOptions(opts); err != nil {
		return nil, err
	}

	return &webhookCheckRun{
		CheckRun:               checkRun,
		webhookCheckRunOptions: opts,
		namespace:              ws.Namespace,
		workspace:              ws.Name,
		login:                  login,
		repo:                   repo,
		path:                   path,
		out:                    new(bytes.Buffer),
	}, nil
}

func reRequestedCheckRun(checkRun *github.CheckRun, path string, opts *webhookCheckRunOptions) (*webhookCheckRun, error) {
	if err := validateCheckRunOptions(opts); err != nil {
		return nil, err
	}

	// Get existing run
	labelSelector := fmt.Sprintf("%s=%s", checkRunLabel, checkRun.ID)
	runs, err := opts.Client.RunsClient("").List(context.Background(), metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil || len(runs.Items) == 0 {
		return nil, fmt.Errorf("unable to find previous etok run: %w")
	}

	// We will get more than one matching Run if previously re-requested...but
	// any Run will do
	run := runs.Items[0]

	return &webhookCheckRun{
		CheckRun:               checkRun,
		workspace:              run.Workspace,
		namespace:              run.Namespace,
		webhookCheckRunOptions: opts,
		login:                  *checkRun.CheckSuite.Repository.Owner.Login,
		repo:                   *checkRun.CheckSuite.Repository.Name,
		path:                   path,
		out:                    new(bytes.Buffer),
	}, nil
}

func (o *webhookCheckRun) handleRun() {
	var conclusion string

	if err := o.run(); err != nil {
		conclusion = "failure"
	} else {
		conclusion = "success"
	}

	_, _, err := o.ghClient.Checks.UpdateCheckRun(context.Background(), o.login, o.repo, *o.CheckSuite.ID, github.UpdateCheckRunOptions{
		Name:       *o.Name,
		HeadSHA:    o.HeadSHA,
		Status:     github.String("completed"),
		Conclusion: github.String(conclusion),
		Output: &github.CheckRunOutput{
			Title: github.String(*o.Name),
			Text:  github.String(o.out.String()),
		},
	})
	if err != nil {
		klog.Errorf("unable to update check run: %s", err.Error())
	}
}

func (o *webhookCheckRun) run() error {
	// Construct launcher options
	launcherOpts := launcher.LauncherOptions{
		Client:     o.Client,
		Workspace:  o.ws.Name,
		Namespace:  o.ws.Namespace,
		DisableTTY: true,
		Command:    "init",
		//Args:       []string{"-c", "shell script goes here"},
		Path: filepath.Join(o.path, o.ws.Spec.VCS.WorkingDir),
		IOStreams: &cmdutil.IOStreams{
			Out: o.out,
		},
	}

	// Allow tests to override run name
	if o.runName != "" {
		launcherOpts.RunName = o.runName
	}

	// Allow tests to override run status
	if o.runStatus != nil {
		launcherOpts.Status = o.runStatus
	}

	// Launch run
	l, err := launcher.NewLauncher(&launcherOpts)
	if err != nil {
		return err
	}
	if err := l.Launch(context.Background()); err != nil {
		return err
	}

	return nil
}
