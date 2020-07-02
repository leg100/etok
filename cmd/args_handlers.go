package cmd

import (
	"fmt"
	"strings"
)

// Wrap any shell args as a 'command string'
func shellWrapArgs(args []string) []string {
	if len(args) > 0 {
		cmdStr := fmt.Sprintf("\"%s\"", strings.Join(args, " "))
		return []string{"-c", cmdStr}
	} else {
		return []string{}
	}
}
