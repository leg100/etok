package workspace

import (
	"fmt"
	"os"

	"github.com/leg100/etok/cmd/flags"
	cmdutil "github.com/leg100/etok/cmd/util"
	"github.com/leg100/etok/pkg/env"
	"github.com/leg100/etok/pkg/repo"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func listCmd(f *cmdutil.Factory) *cobra.Command {
	var path, kubeContext string
	var namespace = defaultNamespace
	var workspace = defaultWorkspace

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List workspaces for current path",
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			// Ensure path is within a git repository
			repo, err := repo.Open(path)
			if err != nil {
				return err
			}

			// Get relative path of root module relative to git repo, so we can
			// compare it to the workspace working dir
			workingDir, err := repo.RootModuleRelativePath()
			if err != nil {
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
				// Eliminate workspaces belonging to (a) other repos, and (b)
				// different working directories
				if ws.Spec.VCS.Repository != repo.Url() {
					continue
				}

				if ws.Spec.VCS.WorkingDir != workingDir {
					continue
				}

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
