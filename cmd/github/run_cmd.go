package github

import (
	"fmt"

	"golang.org/x/sync/errgroup"
	ctrl "sigs.k8s.io/controller-runtime"

	cmdutil "github.com/leg100/etok/cmd/util"
	"github.com/leg100/etok/pkg/scheme"
	"github.com/spf13/cobra"
	"k8s.io/klog/v2/klogr"
)

const (
	defaultWebhookPort = 9001
)

// runOptions are the options for running a github app
type runOptions struct {
	*webhookServer

	appOptions

	// Github app ID
	appID int64

	// Github hostname
	githubHostname string

	// Path to github app private key
	keyPath string
}

// runCmd creates a cobra command for running github app
func runCmd(f *cmdutil.Factory) (*cobra.Command, *runOptions) {
	o := &runOptions{
		webhookServer: &webhookServer{},
	}
	cmd := &cobra.Command{
		Use:    "run",
		Short:  "Run github app",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			// Create k8s client
			client, err := f.Create("")
			if err != nil {
				return err
			}

			// The github client mgr maintains one client per github app
			// installation
			installsMgr, err := newInstallsManager(o.githubHostname, o.keyPath, o.appID)
			if err != nil {
				return err
			}

			ctrl.SetLogger(klogr.NewWithOptions(klogr.WithFormat(klogr.FormatKlog)))

			// Manager for reconcilers
			mgr, err := ctrl.NewManager(client.Config, ctrl.Options{
				Scheme: scheme.Scheme,
			})
			if err != nil {
				return fmt.Errorf("unable to create run controller manager: %w", err)
			}

			if err := newCheckSuiteReconciler(
				mgr.GetClient(),
				installsMgr,
				o.cloneDir,
			).SetupWithManager(mgr); err != nil {
				return fmt.Errorf("unable to create check suite controller: %w", err)
			}

			if err := newCheckReconciler(
				mgr.GetClient(),
				client.KubeClient,
				installsMgr,
				o.stripRefreshing,
			).SetupWithManager(mgr); err != nil {
				return fmt.Errorf("unable to create check run controller: %w", err)
			}

			// The github app
			app := newApp(client.RuntimeClient, o.appOptions)

			// Configure webhook server to forward events to the github app
			o.webhookServer.app = app
			o.webhookServer.mgr = installsMgr

			// Ensure webhook server is properly constructed since we're not
			// using a constructor
			if err := o.webhookServer.validate(); err != nil {
				return err
			}

			// Start controller mgr and webhook server concurrently. If either
			// returns an error, both are cancelled.
			g, gctx := errgroup.WithContext(cmd.Context())

			g.Go(func() error {
				return mgr.Start(gctx)
			})

			g.Go(func() error {
				return o.webhookServer.run(gctx)
			})

			return g.Wait()
		},
	}

	cmd.Flags().StringVar(&o.githubHostname, "hostname", "github.com", "Github hostname")
	cmd.Flags().Int64Var(&o.appID, "app-id", 0, "Github app ID")
	cmd.Flags().StringVar(&o.keyPath, "key-path", "", "Github app private key path")

	cmd.Flags().IntVar(&o.port, "port", defaultWebhookPort, "Webhook port")
	cmd.Flags().StringVar(&o.webhookSecret, "webhook-secret", "", "Github app webhook secret")

	// Default to /repos, the mountpoint of a dedicated k8s volume
	cmd.Flags().StringVar(&o.cloneDir, "clone-path", "/repos", "Path to a directory in which to clone repos")
	cmd.Flags().BoolVar(&o.stripRefreshing, "strip-refreshing", false, "Strip refreshing log lines from terraform plan output")

	return cmd, o
}
