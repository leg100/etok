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
	"os"

	"github.com/apex/log"
	"github.com/leg100/stok/cmd"
	"github.com/leg100/stok/pkg/signals"
)

func main() {
	// Create context, and cancel if interrupt is received
	ctx, cancel := context.WithCancel(context.Background())
	signals.CatchCtrlC(cancel)

	code, err := cmd.ExecWithExitCode(ctx, os.Args[1:], os.Stdout, os.Stderr)
	if err != nil {
		log.WithError(err).Error("Fatal error")

	}
	os.Exit(code)
}
