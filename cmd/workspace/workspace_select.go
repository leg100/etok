package workspace

import (
	"fmt"

	"github.com/leg100/etok/cmd/flags"
	cmdutil "github.com/leg100/etok/cmd/util"
	"github.com/leg100/etok/pkg/env"
	"github.com/leg100/etok/pkg/repo"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func selectCmd(f *cmdutil.Factory) *cobra.Command {
	// Root module path
	var path string
	// Namespace of workspace to select
	var namespace = defaultNamespace
	// k8s context
	var kubeContext string

	cmd := &cobra.Command{
		Use:   "select <workspace>",
		Short: "Select an etok workspace",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Ensure path is within a git repository
			repo, err := repo.Open(path)
			if err != nil {
				return err
			}

			name := args[0]

			// Validates parameters
			etokenv, err := env.New(namespace, name)
			if err != nil {
				return err
			}

			client, err := f.Create(kubeContext)
			if err != nil {
				return err
			}

			// Get workspace, to validate it exists and ensure its valid for
			// this root modoule
			selected, err := client.WorkspacesClient(namespace).Get(cmd.Context(), name, metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("unable to retrieve workspace resource: %w", err)
			}

			// Get relative path of root module relative to git repo, so we can
			// compare it to the workspace working dir
			workingDir, err := repo.RootModuleRelativePath()
			if err != nil {
				return err
			}

			if selected.Spec.VCS.WorkingDir != workingDir {
				return fmt.Errorf("workspace working directory mismatch: '%s' (current) != '%s' (selected)", workingDir, selected.Spec.VCS.WorkingDir)
			}

			if selected.Spec.VCS.Repository != repo.Url() {
				return fmt.Errorf("workspace repository mismatch: '%s' (current) != '%s' (selected)", repo.Url(), selected.Spec.VCS.Repository)
			}

			if err := etokenv.Write(path); err != nil {
				return err
			}
			fmt.Fprintf(f.Out, "Current workspace now: %s\n", etokenv)
			return nil
		},
	}

	flags.AddPathFlag(cmd, &path)
	flags.AddKubeContextFlag(cmd, &kubeContext)
	flags.AddNamespaceFlag(cmd, &namespace)

	return cmd
}
