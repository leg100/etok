package cmd

import (
	"context"

	"github.com/leg100/etok/cmd/envvars"
	cmdutil "github.com/leg100/etok/cmd/util"
)

// ParseArgs parses CLI args and executes the select command
func ParseArgs(ctx context.Context, args []string, opts *cmdutil.Options) error {
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
