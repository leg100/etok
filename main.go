/*
Copyright © 2020 Louis Garman <louisgarman@gmail.com>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/fatih/color"
	"github.com/leg100/stok/cmd"
	"github.com/leg100/stok/pkg/app"
	"github.com/leg100/stok/pkg/errors"
	"github.com/leg100/stok/pkg/signals"
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
	opts, err := app.NewOpts(out, errout, in)
	if err != nil {
		return err
	}

	// Parse args and execute selected command
	return cmd.ParseArgs(ctx, args, opts)
}

// Print error message unless the error originated from executing a program (which would have
// printed its own message)
func handleError(err error, out io.Writer) int {
	var exiterr errors.ExitError
	if errors.As(err, &exiterr) {
		return exiterr.ExitCode()
	}
	fmt.Fprintf(out, "%s %s\n", color.HiRedString("Error:"), err.Error())
	return 1
}
