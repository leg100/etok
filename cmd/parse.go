package cmd

import (
	"context"

	"github.com/leg100/stok/cmd/envvars"
	"github.com/leg100/stok/pkg/app"
)

// ParseArgs parses CLI args and executes the select command
func ParseArgs(ctx context.Context, args []string, opts *app.Options) error {
	// Build root command
	cmd := RootCmd(opts)

	// Override os.Args
	cmd.SetArgs(args)

	// Lookup env vars and override flag defaults
	envvars.SetFlagsFromEnvVariables(cmd)

	// Parse args
	if err := cmd.ExecuteContext(ctx); err != nil {
		return err
	}

	return nil
}
