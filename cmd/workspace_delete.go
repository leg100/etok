package cmd

import (
	"context"
	"fmt"

	"github.com/leg100/stok/cmd/flags"
	"github.com/leg100/stok/pkg/app"
	"github.com/leg100/stok/pkg/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func init() {
	workspaceCmd.AddChild(
		NewCmd("delete <workspace>").
			WithShortHelp("Deletes a stok workspace").
			WithFlags(
				flags.Path,
				flags.Namespace,
			).
			WithOneArg().
			WantsKubeClients().
			WithExec(func(ctx context.Context, opts *app.Options) error {
				ws := opts.Args[0]

				if err := opts.StokClient().StokV1alpha1().Workspaces(opts.Namespace).Delete(ctx, ws, metav1.DeleteOptions{}); err != nil {
					return fmt.Errorf("failed to delete workspace: %w", err)
				}

				log.Infof("deleted workspace %s/%s\n", opts.Namespace, ws)

				return nil
			}))
}
