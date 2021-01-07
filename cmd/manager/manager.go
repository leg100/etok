package manager

import (
	"flag"
	"fmt"
	"runtime"

	"k8s.io/klog/v2"

	"github.com/leg100/etok/cmd/flags"
	cmdutil "github.com/leg100/etok/cmd/util"
	"github.com/leg100/etok/pkg/controllers"
	"github.com/leg100/etok/pkg/scheme"
	"github.com/leg100/etok/pkg/version"
	"github.com/spf13/cobra"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func printVersion() {
	klog.V(0).Info(fmt.Sprintf("Operator Version: %s", version.Version))
	klog.V(0).Info(fmt.Sprintf("Go Version: %s", runtime.Version()))
	klog.V(0).Info(fmt.Sprintf("Go OS/Arch: %s/%s", runtime.GOOS, runtime.GOARCH))
}

type ManagerOptions struct {
	*cmdutil.Factory

	KubeContext string

	// Docker image used for both the operator and the runner
	Image string

	// Operator metrics bind endpoint
	MetricsAddress string
	// Toggle operator leader election
	EnableLeaderElection bool

	args []string
}

func ManagerCmd(f *cmdutil.Factory) *cobra.Command {
	o := &ManagerOptions{Factory: f}
	cmd := &cobra.Command{
		Use:    "operator",
		Short:  "Run the etok operator",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			ctrl.SetLogger(zap.New(zap.UseDevMode(false)))

			printVersion()

			client, err := f.Create(o.KubeContext)
			if err != nil {
				return err
			}

			mgr, err := ctrl.NewManager(client.Config, ctrl.Options{
				Scheme:             scheme.Scheme,
				MetricsBindAddress: o.MetricsAddress,
				Port:               9443,
				LeaderElection:     o.EnableLeaderElection,
				LeaderElectionID:   "688c905b.dev",
			})
			if err != nil {
				return fmt.Errorf("unable to start manager: %w", err)
			}

			klog.V(0).Info("Runner image: " + o.Image)

			// Setup workspace ctrl with mgr
			if err := controllers.NewWorkspaceReconciler(mgr.GetClient(), o.Image).SetupWithManager(mgr); err != nil {
				return fmt.Errorf("unable to create workspace controller: %w", err)
			}

			// Setup run ctrl with mgr
			if err := controllers.NewRunReconciler(mgr.GetClient(), o.Image).SetupWithManager(mgr); err != nil {
				return fmt.Errorf("unable to create run controller: %w", err)
			}

			klog.V(0).Info("starting manager")
			if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
				return fmt.Errorf("problem running manager: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().AddGoFlagSet(flag.CommandLine)

	flags.AddKubeContextFlag(cmd, &o.KubeContext)

	cmd.Flags().StringVar(&o.MetricsAddress, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	cmd.Flags().BoolVar(&o.EnableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	cmd.Flags().StringVar(&o.Image, "image", version.Image, "Docker image used for both the operator and the runner")

	return cmd
}
