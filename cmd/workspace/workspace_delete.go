package workspace

import (
	"fmt"

	"github.com/leg100/stok/cmd/flags"
	cmdutil "github.com/leg100/stok/cmd/util"
	"github.com/leg100/stok/pkg/log"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func DeleteCmd(opts *cmdutil.Options) *cobra.Command {
	var namespace, kubeContext string
	cmd := &cobra.Command{
		Use:   "delete <workspace>",
		Short: "Deletes a stok workspace",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			ws := args[0]

			client, err := opts.Create(kubeContext)
			if err != nil {
				return err
			}

			if err := client.WorkspacesClient(namespace).Delete(cmd.Context(), ws, metav1.DeleteOptions{}); err != nil {
				return fmt.Errorf("failed to delete workspace: %w", err)
			}

			log.Infof("deleted workspace %s/%s\n", namespace, ws)

			return nil
		},
	}

	flags.AddNamespaceFlag(cmd, &namespace)
	flags.AddKubeContextFlag(cmd, &kubeContext)

	return cmd
}
