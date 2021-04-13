package github

import (
	"context"

	"github.com/google/go-github/v31/github"
)

// checkRunOperation handles creating and updating a github check run for an etok
// run
type checkRunOperation struct {
	actions []*github.CheckRunAction

	*checkRun
}

func (c *checkRunOperation) setAction(label, desc, id string) {
	c.actions = append(c.actions, &github.CheckRunAction{Label: label, Description: desc, Identifier: id})
}

func (c *checkRunOperation) output() *github.CheckRunOutput {
	return &github.CheckRunOutput{
		Title:   github.String(c.title()),
		Summary: github.String(c.summary()),
		Text:    github.String(c.details()),
	}
}

// create new check run
func (c *checkRunOperation) create(ctx context.Context, client *GithubClient) (int64, error) {
	opts := github.CreateCheckRunOptions{
		Name:       c.name(),
		HeadSHA:    c.sha,
		Status:     c.status,
		Conclusion: c.conclusion,
		Output:     c.output(),
		Actions:    c.actions,
		ExternalID: c.externalID(),
	}

	checkRun, _, err := client.Checks.CreateCheckRun(ctx, c.owner, c.repo, opts)
	if err != nil {
		return 0, err
	}
	return *checkRun.ID, nil
}

// update existing check run
func (c *checkRunOperation) update(ctx context.Context, client *GithubClient, id int64) error {
	opts := github.UpdateCheckRunOptions{
		Name:       c.name(),
		HeadSHA:    github.String(c.sha),
		Status:     c.status,
		Conclusion: c.conclusion,
		Output:     c.output(),
		Actions:    c.actions,
		ExternalID: c.externalID(),
	}

	_, _, err := client.Checks.UpdateCheckRun(ctx, c.owner, c.repo, id, opts)
	return err
}
