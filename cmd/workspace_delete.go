package cmd

import (
	"context"
	"fmt"

	"github.com/apex/log"
	"github.com/leg100/stok/cmd/flags"
	"github.com/leg100/stok/pkg/env"
	"github.com/leg100/stok/pkg/options"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func init() {
	workspaceCmd.AddChild(
		NewCmd("delete").
			WithShortUsage("delete <[namespace/]workspace>").
			WithShortHelp("Deletes a stok workspace").
			WithFlags(
				flags.Path,
				flags.Namespace,
			).
			WithOneArg().
			WantsKubeClients().
			WithExec(func(ctx context.Context, opts *options.StokOptions) error {
				name, ns, err := env.ValidateAndParse(opts.Args[0])
				if err != nil {
					return err
				}

				if ns != "" {
					// Arg includes namespace; override flag value
					opts.Namespace = ns
				}

				objs, _ := opts.StokClient.StokV1alpha1().Workspaces("").List(ctx, metav1.ListOptions{})
				fmt.Printf("%#v\n", objs.Items)

				if err := opts.StokClient.StokV1alpha1().Workspaces(opts.Namespace).Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
					return fmt.Errorf("failed to delete workspace: %w", err)
				}

				log.WithFields(log.Fields{
					"workspace": name,
					"namespace": ns,
				}).Info("Deleted workspace")

				return nil
			}))
}
