package github

// There are only two check run commands:
var (
	planCmd  = checkRunCommand("plan")
	applyCmd = checkRunCommand("apply")
)

// Command to be run on behalf of check run
type checkRunCommand string
