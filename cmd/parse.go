package cmd

import (
	"context"

	"github.com/leg100/stok/pkg/app"
	"github.com/leg100/stok/pkg/log"
)

// ParseArgs parses CLI args and furnishes the factory f with a selected app to be run
func ParseArgs(ctx context.Context, args []string, opts *app.Options) error {
	// Build command tree
	cmd := root.Build(opts, true)

	// Override os.Args
	cmd.SetArgs(args)

	// Lookup env vars and override flag defaults
	setFlagsFromEnvVariables(cmd)

	// Parse args
	if err := cmd.ExecuteContext(ctx); err != nil {
		return err
	}

	if opts.Debug {
		log.SetLevel(log.DebugLevel)
		log.Debug("Debug logging enabled")
	}

	return nil
}
