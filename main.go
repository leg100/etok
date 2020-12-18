package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	"github.com/fatih/color"
	"github.com/leg100/etok/cmd"
	cmdutil "github.com/leg100/etok/cmd/util"
	etokerrors "github.com/leg100/etok/pkg/errors"
	"github.com/leg100/etok/pkg/signals"
)

func main() {
	// Exit code
	var code int

	if err := run(os.Args[1:], os.Stdout, os.Stderr, os.Stdin); err != nil {
		code = handleError(err, os.Stderr)
	}
	os.Exit(code)
}

func run(args []string, out, errout io.Writer, in io.Reader) error {
	// Create context, and cancel if interrupt is received
	ctx, cancel := context.WithCancel(context.Background())
	signals.CatchCtrlC(cancel)

	// Construct options and their defaults
	opts, err := cmdutil.NewOpts(out, errout, in)
	if err != nil {
		return err
	}

	// Parse args and execute selected command
	return cmd.ParseArgs(ctx, args, opts)
}

// Print error message unless the error originated from executing a program (which would have
// printed its own message)
func handleError(err error, out io.Writer) int {
	var exit etokerrors.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode()
	}
	fmt.Fprintf(out, "%s %s\n", color.HiRedString("Error:"), err.Error())
	return 1
}
