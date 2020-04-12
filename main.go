package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/leg100/stok/app"
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

const (
	exitCodeErr       = 1
	exitCodeInterrupt = 2
	exitCodeClient    = 3
)

var out io.Writer = os.Stdout
var stderr io.Writer = os.Stderr

func main() {
	args := os.Args[1:]

	if is_local(args) {
		handleError(runLocal(args))
	} else {
		if err := runRemote(args); err != nil {
			fmt.Fprint(stderr, err)
			os.Exit(exitCodeClient)
		}
	}
}

func is_local(args []string) bool {
	if len(args) > 0 && slice.ContainsString(TERRAFORM_COMMANDS_THAT_USE_STATE, args[0]) {
		return false
	}
	return true
}

func runLocal(args []string) error {
	cmd := exec.Command("terraform", args...)

	cmd.Stdout = out
	cmd.Stderr = stderr

	return cmd.Run()
}

func handleError(err error) {
	if exiterr, ok := err.(*exec.ExitError); ok {
		// terraform exited with non-zero exit code
		os.Exit(exiterr.ExitCode())
	}
	if err != nil {
		fmt.Fprintf(stderr, "%s\n", err)
		os.Exit(exitCodeErr)
	}
}

func runRemote(args []string) error {
	client, kubeClient, err := app.InitClient()
	if err != nil {
		return err
	}

	workspace := os.Getenv("STOK_WORKSPACE")
	if workspace == "" {
		workspace = "default"
	}

	namespace := os.Getenv("STOK_NAMESPACE")
	if namespace == "" {
		namespace = "default"
	}

	app := &app.App{
		Namespace:  namespace,
		Workspace:  workspace,
		Args:       args,
		Client:     *client,
		KubeClient: kubeClient,
	}
	return app.Run()
}
