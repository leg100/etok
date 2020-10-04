package cmd

import (
	"context"
	"fmt"

	"github.com/leg100/stok/cmd/flags"
	"github.com/leg100/stok/pkg/env"
	"github.com/leg100/stok/pkg/options"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func init() {
	workspaceCmd.AddChild(
		NewCmd("list").
			WithShortUsage("stok workspace list [flags]").
			WithShortHelp("List all workspaces").
			WithFlags(
				flags.Path,
			).
			WithOneArg().
			WantsKubeClients().
			WithExec(func(ctx context.Context, opts *options.StokOptions) error {
				stokenv, err := env.ReadStokEnv(opts.Path)
				if err != nil {
					return err
				}

				// List across all namespaces
				workspaces, err := opts.StokClient.StokV1alpha1().Workspaces("").List(ctx, metav1.ListOptions{})
				if err != nil {
					return err
				}

				var prefix string
				for _, ws := range workspaces.Items {
					if ws.GetNamespace() == stokenv.Namespace() && ws.GetName() == stokenv.Workspace() {
						prefix = "*"
					} else {
						prefix = ""
					}
					fmt.Fprintf(opts.Out, "%s\t%s/%s\n", prefix, ws.GetNamespace(), ws.GetName())
				}

				return nil
			}))
}
