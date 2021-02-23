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
	*client.Client
	kubeContext string

	// We'll need this when we deploy k8s resources
	namespace string

	*webhookServer
}

// runCmd creates a cobra command for running a github app
func runCmd(f *cmdutil.Factory) (*cobra.Command, *runOptions) {
	o := &runOptions{
		namespace:     defaultNamespace,
		webhookServer: &webhookServer{},
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

			if err := o.webhookServer.run(cmd.Context()); err != nil {
				return err
			}

			return nil
		},
	}

	flags.AddNamespaceFlag(cmd, &o.namespace)
	flags.AddKubeContextFlag(cmd, &o.kubeContext)

	cmd.Flags().StringVar(&o.githubHostname, "hostname", "github.com", "Github hostname")
	cmd.Flags().Int64Var(&o.appID, "app-id", 0, "Github app ID")
	cmd.Flags().StringVar(&o.keyPath, "key-path", "", "Github app private key path")

	cmd.Flags().BytesHexVar(&o.webhookSecret, "webhook-secret", nil, "Github app webhook secret")

	cmd.Flags().IntVar(&o.port, "port", defaultWebhookServerPort, "Webhook port")

	return cmd, o
}
