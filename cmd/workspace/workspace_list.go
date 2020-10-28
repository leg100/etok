package workspace

import (
	"fmt"
	"os"

	"github.com/leg100/stok/cmd/flags"
	"github.com/leg100/stok/pkg/app"
	"github.com/leg100/stok/pkg/env"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func ListCmd(opts *app.Options) *cobra.Command {
	var path, kubeContext, namespace string
	var workspace = "default"

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all workspaces",
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			client, err := opts.Create(kubeContext)
			if err != nil {
				return err
			}

			stokenv, err := env.ReadStokEnv(path)
			if err != nil {
				if !os.IsNotExist(err) {
					return err
				}
			} else {
				namespace = stokenv.Namespace()
				workspace = stokenv.Workspace()
			}

			// List across all namespaces
			workspaces, err := client.WorkspacesClient("").List(cmd.Context(), metav1.ListOptions{})
			if err != nil {
				return err
			}

			var prefix string
			for _, ws := range workspaces.Items {
				if ws.GetNamespace() == namespace && ws.GetName() == workspace {
					prefix = "*"
				} else {
					prefix = ""
				}
				fmt.Fprintf(opts.Out, "%s\t%s/%s\n", prefix, ws.GetNamespace(), ws.GetName())
			}

			return nil
		},
	}

	flags.AddPathFlag(cmd, &path)
	flags.AddNamespaceFlag(cmd, &namespace)
	flags.AddKubeContextFlag(cmd, &kubeContext)

	return cmd
}
