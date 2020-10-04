package cmd

import (
	"flag"
	"time"

	"github.com/leg100/stok/cmd/flags"
	"github.com/leg100/stok/pkg/apps/workspace"
	"github.com/leg100/stok/pkg/env"
	"github.com/leg100/stok/pkg/options"
)

func init() {
	workspaceCmd.AddChild(
		NewCmd("new").
			WithShortUsage("new <[namespace/]workspace>").
			WithShortHelp("Create a new stok workspace").
			WithFlags(
				flags.Path,
				workspaceFlags,
			).
			WithOneArg().
			WithPreExec(func(fs *flag.FlagSet, opts *options.StokOptions) error {
				name, ns, err := env.ValidateAndParse(opts.Args[0])
				if err != nil {
					return err
				}

				if ns != "" {
					// Namespace arg overrides namespace flag
					opts.Namespace = ns
				}

				opts.Name = name

				return nil
			}).
			WithApp(workspace.NewFromOptions))
}

func workspaceFlags(fs *flag.FlagSet, opts *options.StokOptions) {
	fs.BoolVar(&opts.CreateServiceAccount, "create-service-account", true, "Create service account if it does not exist")
	fs.BoolVar(&opts.CreateSecret, "create-secret", true, "Create secret if it does not exist")

	fs.StringVar(&opts.WorkspaceSpec.ServiceAccountName, "service-account", "stok", "Name of ServiceAccount")
	fs.StringVar(&opts.WorkspaceSpec.SecretName, "secret", "stok", "Name of Secret containing credentials")
	fs.StringVar(&opts.WorkspaceSpec.Cache.Size, "size", "1Gi", "Size of PersistentVolume for cache")
	fs.StringVar(&opts.WorkspaceSpec.Cache.StorageClass, "storage-class", "", "StorageClass of PersistentVolume for cache")
	fs.StringVar(&opts.WorkspaceSpec.TimeoutClient, "timeout-client", "10s", "timeout for client to signal readiness")
	fs.StringVar(&opts.WorkspaceSpec.Backend.Type, "backend-type", "local", "Set backend type")

	fs.DurationVar(&opts.TimeoutWorkspace, "timeout", 10*time.Second, "Time to wait for workspace to be healthy")
	fs.DurationVar(&opts.TimeoutWorkspacePod, "timeout-pod", time.Minute, "timeout for pod to be ready and running")

	opts.WorkspaceSpec.Backend.Config = *flags.StringToStringFlag(fs, "backend-config", map[string]string{}, "Set backend config (command separated key values, e.g. bucket=gcs,prefix=dev")
}
