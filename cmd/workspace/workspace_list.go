package workspace

import (
	"fmt"
	"os"

	"github.com/go-git/go-git/v5"
	"github.com/leg100/etok/cmd/flags"
	cmdutil "github.com/leg100/etok/cmd/util"
	"github.com/leg100/etok/pkg/env"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func listCmd(f *cmdutil.Factory) *cobra.Command {
	var path, kubeContext string
	var namespace = defaultNamespace
	var workspace = defaultWorkspace

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all workspaces",
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			_, err = openRepo(path)
			if err != nil {
				if err == git.ErrRepositoryNotExists {
					return errRepositoryNotFound
				}
				return err
			}

			client, err := f.Create(kubeContext)
			if err != nil {
				return err
			}

			etokenv, err := env.Read(path)
			if err != nil {
				if !os.IsNotExist(err) {
					return err
				}
			} else {
				// Override defaults
				namespace = etokenv.Namespace
				workspace = etokenv.Workspace
			}

			// List across all namespaces
			workspaces, err := client.WorkspacesClient("").List(cmd.Context(), metav1.ListOptions{})
			if err != nil {
				return err
			}

			var prefix string
			for _, ws := range workspaces.Items {
				if ws.Namespace == namespace && ws.Name == workspace {
					prefix = "*"
				} else {
					prefix = ""
				}
				fmt.Fprintf(f.Out, "%s\t%s\n", prefix, &env.Env{Namespace: ws.Namespace, Workspace: ws.Name})
			}

			return nil
		},
	}

	flags.AddPathFlag(cmd, &path)
	flags.AddKubeContextFlag(cmd, &kubeContext)

	return cmd
}
