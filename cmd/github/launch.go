package github

import (
	"bytes"
	"context"

	"github.com/google/go-github/v31/github"
	"github.com/leg100/etok/api/etok.dev/v1alpha1"
	cmdutil "github.com/leg100/etok/cmd/util"
	"github.com/leg100/etok/pkg/client"
	"github.com/leg100/etok/pkg/launcher"
	"k8s.io/klog/v2"
)

const (
	checkRunLabel = "etok.dev/github-check-run-id"
)

type checkRunOptions struct {
	runName   string
	runStatus *v1alpha1.RunStatus
}

func launch(checkRun *github.CheckRun, ghClient *GithubClient, kClient *client.Client, path, workspace, namespace string, opts *checkRunOptions) {
	out := new(bytes.Buffer)

	// Construct launcher options
	launcherOpts := launcher.LauncherOptions{
		Client:     kClient,
		Workspace:  workspace,
		Namespace:  namespace,
		DisableTTY: true,
		Command:    "plan",
		//Args:       []string{"-c", "shell script goes here"},
		Path: path,
		IOStreams: &cmdutil.IOStreams{
			Out: out,
		},
	}

	// Allow tests to override run name
	if opts.runName != "" {
		launcherOpts.RunName = opts.runName
	}

	// Allow tests to override run status
	if opts.runStatus != nil {
		launcherOpts.Status = opts.runStatus
	}

	var conclusion, text string
	err := launcher.NewLauncher(&launcherOpts).Launch(context.Background())
	if err != nil {
		conclusion = "failure"
		text = err.Error()
	} else {
		conclusion = "success"
		text = out.String()
	}

	_, _, err = ghClient.Checks.UpdateCheckRun(context.Background(), *checkRun.CheckSuite.Repository.Owner.Login, *checkRun.CheckSuite.Repository.Name, *checkRun.CheckSuite.ID, github.UpdateCheckRunOptions{
		Name:       *checkRun.Name,
		HeadSHA:    checkRun.HeadSHA,
		Status:     github.String("completed"),
		Conclusion: github.String(conclusion),
		Output: &github.CheckRunOutput{
			Title: checkRun.Name,
			Text:  github.String(text),
		},
	})
	if err != nil {
		klog.Errorf("unable to update check run: %s", err.Error())
	}
}
