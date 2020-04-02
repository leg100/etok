package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/leg100/stok/util/slice"
)

var TERRAFORM_COMMANDS_THAT_USE_STATE = []string{
	"init",
	"apply",
	"destroy",
	"env",
	"import",
	"graph",
	"output",
	"plan",
	"push",
	"refresh",
	"show",
	"taint",
	"untaint",
	"validate",
	"force-unlock",
	"state",
}

func main() {
	err := parseArgs(os.Args[1:])
	if err != nil {
		handleError(err)
	}
}

func parseArgs(args []string) error {
	if len(args) > 0 && slice.ContainsString(TERRAFORM_COMMANDS_THAT_USE_STATE, args[0]) {
		fmt.Println("TODO: run remotely")
		return nil
	}

	return runLocal(args)
}

func runLocal(args []string) error {
	cmd := exec.Command("terraform", args...)

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
