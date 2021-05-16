package github

const (
	checkSuiteIDLabelName = "etok.dev/github-checksuite-id"

	checkIDLabelName        = "etok.dev/github-check-id"
	checkStatusLabelName    = "etok.dev/github-check-status"
	checkCommandLabelName   = "etok.dev/github-check-command"
	checkSHALabelName       = "etok.dev/github-check-sha"
	checkOwnerLabelName     = "etok.dev/github-check-owner"
	checkRepoLabelName      = "etok.dev/github-check-repo"
	checkAppliableLabelName = "etok.dev/github-check-appliable"

	githubTriggeredLabelName = "etok.dev/github-triggered"

	// Add install id to run so that the run reconciler knows which github
	// client to use for a given run
	githubAppInstallIDLabelName = "etok.dev/github-app-install-id"
)
