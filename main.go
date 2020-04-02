package main

import (
	"os"
	"os/exec"
)

func main() {
	err := parseArgs(os.Args)
	if err != nil {
		handleError(err)
	}
}

func parseArgs(args []string) error {
	cmd := exec.Command("terraform", args[1:]...)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	return err
}

func handleError(err error) {
	if exiterr, ok := err.(*exec.ExitError); ok {
		// terraform exited with non-zero exit code
		os.Exit(exiterr.ExitCode())
	}
}
