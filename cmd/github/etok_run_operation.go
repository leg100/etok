package github

import (
	"context"

	"github.com/google/go-github/v31/github"
)

// etokRunOperation handles creating and updating a github check run for an etok
// run
type etokRunOperation struct {
	actions    []*github.CheckRunAction
	conclusion *string

	*etokRun
}

func (c *etokRunOperation) setAction(label, desc, id string) {
	c.actions = append(c.actions, &github.CheckRunAction{Label: label, Description: desc, Identifier: id})
}

func (c *etokRunOperation) output() *github.CheckRunOutput {
	return &github.CheckRunOutput{
		Title:   github.String(c.title()),
		Summary: github.String(c.summary()),
		Text:    github.String(c.details()),
	}
}

// create new check run
func (c *etokRunOperation) create(ctx context.Context, client *GithubClient) (int64, error) {
	opts := github.CreateCheckRunOptions{
		Name:       c.name(),
		HeadSHA:    c.repo.sha,
		Status:     c.status(),
		Conclusion: c.conclusion,
		Output:     c.output(),
		Actions:    c.actions,
		// Retain reference to etok run id in case user wants to re-run it
		ExternalID: c.externalID(),
	}

	checkRun, _, err := client.Checks.CreateCheckRun(ctx, c.repo.owner, c.repo.name, opts)
	if err != nil {
		return 0, err
	}
	return *checkRun.ID, nil
}

// update existing check run
func (c *etokRunOperation) update(ctx context.Context, client *GithubClient, id int64) error {
	opts := github.UpdateCheckRunOptions{
		Name:       c.name(),
		HeadSHA:    github.String(c.repo.sha),
		Status:     c.status(),
		Conclusion: c.conclusion,
		Output:     c.output(),
		Actions:    c.actions,
		// Retain reference to etok run id in case user wants to re-run it
		ExternalID: c.externalID(),
	}
	_, _, err := client.Checks.UpdateCheckRun(ctx, c.repo.owner, c.repo.name, id, opts)
	return err
}
