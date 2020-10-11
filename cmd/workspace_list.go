package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/leg100/stok/cmd/flags"
	"github.com/leg100/stok/pkg/app"
	"github.com/leg100/stok/pkg/env"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func init() {
	workspaceCmd.AddChild(
		NewCmd("list [flags]").
			WithShortHelp("List all workspaces").
			WithFlags(
				flags.Path,
			).
			WantsKubeClients().
			WithExec(func(ctx context.Context, opts *app.Options) error {
				stokenv, err := env.ReadStokEnv(opts.Path)
				if err != nil {
					if !os.IsNotExist(err) {
						return err
					}
				} else {
					opts.Namespace = stokenv.Namespace()
					opts.Workspace = stokenv.Workspace()
				}

				// List across all namespaces
				workspaces, err := opts.StokClient().StokV1alpha1().Workspaces("").List(ctx, metav1.ListOptions{})
				if err != nil {
					return err
				}

				var prefix string
				for _, ws := range workspaces.Items {
					if ws.GetNamespace() == opts.Namespace && ws.GetName() == opts.Workspace {
						prefix = "*"
					} else {
						prefix = ""
					}
					fmt.Fprintf(opts.Out, "%s\t%s/%s\n", prefix, ws.GetNamespace(), ws.GetName())
				}

				return nil
			}))
}
