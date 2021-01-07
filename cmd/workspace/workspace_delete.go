package workspace

import (
	"fmt"
	"time"

	"github.com/leg100/etok/cmd/flags"
	cmdutil "github.com/leg100/etok/cmd/util"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

func deleteCmd(opts *cmdutil.Options) *cobra.Command {
	var kubeContext string
	var namespace = defaultNamespace

	cmd := &cobra.Command{
		Use:   "delete <workspace>",
		Short: "Deletes an etok workspace",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ws := args[0]

			client, err := opts.Create(kubeContext)
			if err != nil {
				return err
			}

			if err := client.WorkspacesClient(namespace).Delete(cmd.Context(), ws, metav1.DeleteOptions{}); err != nil {
				return fmt.Errorf("failed to delete workspace: %w", err)
			}

			fmt.Println("Waiting for workspace and its dependent resources to be deleted...")
			wait.PollImmediate(500*time.Millisecond, 60*time.Second, func() (bool, error) {
				if _, err := client.WorkspacesClient(namespace).Get(cmd.Context(), ws, metav1.GetOptions{}); err != nil {
					if errors.IsNotFound(err) {
						return true, nil
					}
					return false, fmt.Errorf("waiting for workspace to be deleted: %w", err)
				}
				return false, nil
			})

			fmt.Printf("Deleted workspace %s/%s\n", namespace, ws)

			return nil
		},
	}

	flags.AddNamespaceFlag(cmd, &namespace)
	flags.AddKubeContextFlag(cmd, &kubeContext)

	return cmd
}
