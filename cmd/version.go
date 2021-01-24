package cmd

import (
	"fmt"

	"github.com/leg100/etok/cmd/flags"
	cmdutil "github.com/leg100/etok/cmd/util"
	"github.com/leg100/etok/pkg/version"
	"github.com/spf13/cobra"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func versionCmd(f *cmdutil.Factory) *cobra.Command {
	// Default namespace of server installation
	var namespace = "etok"
	// Default name of server deployment
	var name = "etok"
	// k8s context
	var kubeContext string

	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Print client version
			fmt.Fprintf(f.Out, "Client Version: %s\t%s\n", version.Version, version.Commit)

			// Try and print server version
			client, err := f.Create(kubeContext)
			if err != nil {
				return err
			}

			deploy, err := client.KubeClient.AppsV1().Deployments(namespace).Get(cmd.Context(), name, metav1.GetOptions{})
			if kerrors.IsNotFound(err) {
				fmt.Fprintf(f.Out, "Server Version: deployment %s/%s not found\n", namespace, name)
				return nil
			}
			if err != nil {
				return fmt.Errorf("unable to determine server version: %w", err)
			}

			lbls := deploy.GetLabels()
			if lbls == nil {
				return fmt.Errorf("unexpectedly found no labels on server deployment")
			}

			v, ok := lbls["version"]
			if !ok {
				return fmt.Errorf("version label missing on server deployment")
			}

			c, ok := lbls["commit"]
			if !ok {
				return fmt.Errorf("commit label missing on server deployment")
			}

			fmt.Fprintf(f.Out, "Server Version: %s\t%s\n", v, c)

			return nil
		},
	}

	flags.AddKubeContextFlag(cmd, &kubeContext)
	cmd.Flags().StringVarP(&namespace, "namespace", "n", namespace, "Kubernetes namespace of server installation")
	cmd.Flags().StringVar(&name, "name", name, "Name of server deployment")

	return cmd
}
