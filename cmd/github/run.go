package github

import (

	// or "gopkg.in/unrolled/render.v1"

	"github.com/leg100/etok/cmd/flags"
	cmdutil "github.com/leg100/etok/cmd/util"
	"github.com/leg100/etok/pkg/client"
	"github.com/spf13/cobra"
)

const (
	defaultWebhookServerPort = 9001
)

// runOptions are the options for running a github app
type runOptions struct {
	*cmdutil.Factory

	*client.Client

	namespace   string
	kubeContext string

	*webhookServerOptions
}

// runCmd creates a cobra command for running a github app
func runCmd(f *cmdutil.Factory) (*cobra.Command, *runOptions) {
	o := &runOptions{
		Factory:              f,
		namespace:            defaultNamespace,
		webhookServerOptions: &webhookServerOptions{},
	}
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run github app",
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			// Create k8s client
			o.Client, err = f.Create(o.kubeContext)
			if err != nil {
				return err
			}

			if err := o.webhookServerOptions.run(cmd.Context()); err != nil {
				return err
			}

			return nil
		},
	}

	flags.AddNamespaceFlag(cmd, &o.namespace)
	flags.AddKubeContextFlag(cmd, &o.kubeContext)

	cmd.Flags().StringVar(&o.githubHostname, "hostname", "github.com", "Github hostname")
	cmd.Flags().Int64Var(&o.creds.AppID, "app-id", 0, "Github app ID")
	cmd.Flags().StringVar(&o.creds.KeyPath, "key-path", "", "Github app private key path")

	cmd.Flags().BytesHexVar(&o.webhookSecret, "webhook-secret", nil, "Github app webhook secret")

	cmd.Flags().IntVar(&o.port, "port", defaultWebhookServerPort, "Webhook port")

	return cmd, o
}
