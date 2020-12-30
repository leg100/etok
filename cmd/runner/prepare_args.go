package runner

import "strings"

// PrepareArgs manipulates the given args depending on the given command
func prepareArgs(command string, args ...string) []string {
	switch command {
	case "sh":
		// Wrap shell args into a single command string
		if len(args) > 0 {
			return []string{"sh", "-c", strings.Join(args, " ")}
		} else {
			return []string{"sh"}
		}
	default:
		// all other commands are actually terraform subcommands
		parts := []string{"terraform"}

		// some commands with spaces in such as 'state pull' need to be
		// separated into separate strings in order to be executed correctly
		parts = append(parts, strings.Split(command, " ")...)
		return append(parts, args...)
	}
}
