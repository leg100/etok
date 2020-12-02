package check

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	cmdutil "github.com/leg100/etok/cmd/util"
	"github.com/spf13/cobra"
)

type CheckOptions struct {
	*cmdutil.Options
}

func CheckCmd(opts *cmdutil.Options) *cobra.Command {
	var kubeContext string
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Check components of etok are running",
		Long:  "Check checks each of the necessary components of etok are running.",
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			client, err := opts.Create(kubeContext)
			if err != nil {
				return err
			}

			operatorLabelSelector := "app=etok, component=operator"
			ctx := cmd.Context()
			deployments, err := client.KubeClient.AppsV1().Deployments("").List(ctx, metav1.ListOptions{LabelSelector: operatorLabelSelector})
			if err != nil {
				return err
			}

			if len(deployments.Items) > 0 {
				fmt.Printf("etok operator version %s deployed\n", deployments.Items[0].Labels["version"])
			}
			return nil
		},
	}
	return cmd
}
