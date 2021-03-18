package github

import (

	// or "gopkg.in/unrolled/render.v1"

	cmdutil "github.com/leg100/etok/cmd/util"
	"github.com/leg100/etok/pkg/logstreamer"
	"github.com/spf13/cobra"
)

const (
	defaultWebhookPort = 9001
)

// runOptions are the options for running a github app
type runOptions struct {
	*webhookServer

	etokAppOptions
}

// runCmd creates a cobra command for running github app
func runCmd(f *cmdutil.Factory) (*cobra.Command, *runOptions) {
	o := &runOptions{}
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run github app",
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			// Create k8s client
			client, err := f.Create("")
			if err != nil {
				return err
			}

			// Set func to use for streaming logs from a run's pod
			o.etokAppOptions.getLogsFunc = logstreamer.GetLogs

			app := newEtokRunApp(client, o.etokAppOptions)

			o.webhookServer = newWebhookServer(app)

			if err := o.webhookServer.validate(); err != nil {
				return err
			}

			if err := o.webhookServer.run(cmd.Context()); err != nil {
				return err
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&o.githubHostname, "hostname", "github.com", "Github hostname")
	cmd.Flags().Int64Var(&o.appID, "app-id", 0, "Github app ID")
	cmd.Flags().StringVar(&o.keyPath, "key-path", "", "Github app private key path")

	cmd.Flags().IntVar(&o.port, "port", defaultWebhookPort, "Webhook port")
	cmd.Flags().StringVar(&o.webhookSecret, "webhook-secret", "", "Github app webhook secret")

	cmd.Flags().StringVar(&o.cloneDir, "clone-path", "", "Path to a directory in which to clone repos")
	cmd.Flags().BoolVar(&o.stripRefreshing, "strip-refreshing", false, "Strip refreshing log lines from terraform plan output")

	return cmd, o
}
