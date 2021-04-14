package github

const (
	checkSuiteIDLabelName = "etok.dev/github-checksuite-id"

	checkRunIDLabelName        = "etok.dev/github-checkrun-id"
	checkRunStatusLabelName    = "etok.dev/github-checkrun-status"
	checkRunCommandLabelName   = "etok.dev/github-checkrun-command"
	checkRunSHALabelName       = "etok.dev/github-checkrun-sha"
	checkRunOwnerLabelName     = "etok.dev/github-checkrun-owner"
	checkRunRepoLabelName      = "etok.dev/github-checkrun-repo"
	checkRunIterationLabelName = "etok.dev/github-checkrun-iteration"

	githubTriggeredLabelName = "etok.dev/github-triggered"

	// Add install id to run so that the run reconciler knows which github
	// client to use for a given run
	githubAppInstallIDLabelName = "etok.dev/github-app-install-id"
)
