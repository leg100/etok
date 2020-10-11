package cmd

import (
	"context"
	"fmt"
	"runtime"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"github.com/leg100/stok/controllers"
	"github.com/leg100/stok/pkg/app"
	"github.com/leg100/stok/pkg/log"
	"github.com/leg100/stok/scheme"
	"github.com/leg100/stok/version"
	"github.com/spf13/pflag"

	sdkVersion "github.com/operator-framework/operator-sdk/version"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func printVersion() {
	log.Info(fmt.Sprintf("Operator Version: %s", version.Version))
	log.Info(fmt.Sprintf("Go Version: %s", runtime.Version()))
	log.Info(fmt.Sprintf("Go OS/Arch: %s/%s", runtime.GOOS, runtime.GOARCH))
	log.Info(fmt.Sprintf("Version of operator-sdk: %v", sdkVersion.Version))
}

func init() {
	root.AddChild(NewCmd("operator").
		WithShortHelp("Run the stok operator").
		WithHidden().
		WithFlags(
			func(fs *pflag.FlagSet, opts *app.Options) {
				fs.StringVar(&opts.MetricsAddress, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
				fs.BoolVar(&opts.EnableLeaderElection, "enable-leader-election", false,
					"Enable leader election for controller manager. "+
						"Enabling this will ensure there is only one active controller manager.")
				fs.StringVar(&opts.Image, "image", version.Image, "Docker image used for both the operator and the runner")
			},
		).
		WithExec(func(ctx context.Context, opts *app.Options) error {
			ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

			printVersion()

			mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
				Scheme:             scheme.Scheme,
				MetricsBindAddress: opts.MetricsAddress,
				Port:               9443,
				LeaderElection:     opts.EnableLeaderElection,
				LeaderElectionID:   "688c905b.goalspike.com",
			})
			if err != nil {
				return fmt.Errorf("unable to start manager: %w", err)
			}

			log.Info("Runner image: " + opts.Image)

			// Setup workspace ctrl with mgr
			if err := controllers.NewWorkspaceReconciler(mgr.GetClient(), opts.Image).SetupWithManager(mgr); err != nil {
				return fmt.Errorf("unable to create workspace controller: %w", err)
			}

			// Setup run ctrl with mgr
			if err := controllers.NewRunReconciler(mgr.GetClient(), opts.Image).SetupWithManager(mgr); err != nil {
				return fmt.Errorf("unable to create run controller: %w", err)
			}

			log.Info("starting manager")
			if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
				return fmt.Errorf("problem running manager: %w", err)
			}

			return nil
		}))
}
