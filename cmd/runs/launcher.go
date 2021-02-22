package runs

import (
	"fmt"
	"os"
	"strings"

	"github.com/leg100/etok/api/etok.dev/v1alpha1"
	"github.com/leg100/etok/cmd/flags"
	cmdutil "github.com/leg100/etok/cmd/util"
	"github.com/leg100/etok/pkg/env"
	"github.com/leg100/etok/pkg/launcher"
	"github.com/leg100/etok/pkg/util"
	"github.com/spf13/cobra"
)

// runCommand constructs a cobra command for launching a 'run', i.e. creating
// the Run resource and its dependencies, and watching its progress, streaming
// its logs, etc.
func runCommand(f *cmdutil.Factory, o *launcher.LauncherOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   fmt.Sprintf("%s [flags] -- [%[1]s args]", strings.Fields(o.Command)[len(strings.Fields(o.Command))-1]),
		Short: fmt.Sprintf("Run terraform %s", o.Command),
		RunE: func(cmd *cobra.Command, args []string) error {
			o.Args = args

			// Tests override run name
			if o.RunName == "" {
				o.RunName = fmt.Sprintf("run-%s", util.GenerateRandomString(5))
			}

			var err error
			o.Client, err = f.Create(o.KubeContext)
			if err != nil {
				return err
			}

			// Copy across factory fields to launcher options
			o.AttachFunc = f.AttachFunc
			o.GetLogsFunc = f.GetLogsFunc
			o.IOStreams = &f.IOStreams
			o.Verbosity = f.Verbosity

			// Override namespace and workspace from env file values
			envFile, err := lookupEnvFile(cmd, o.Path)
			if err != nil {
				return err
			}
			if envFile != nil {
				if !flags.IsFlagPassed(cmd.Flags(), "namespace") {
					o.Namespace = envFile.Namespace
				}
				if !flags.IsFlagPassed(cmd.Flags(), "workspace") {
					o.Workspace = envFile.Workspace
				}
			}

			l, err := launcher.NewLauncher(o)
			if err != nil {
				return err
			}

			return l.Launch(cmd.Context())
		},
	}

	flags.AddPathFlag(cmd, &o.Path)
	flags.AddKubeContextFlag(cmd, &o.KubeContext)
	flags.AddDisableResourceCleanupFlag(cmd, &o.DisableResourceCleanup)

	o.Namespace = launcher.DefaultNamespace
	flags.AddNamespaceFlag(cmd, &o.Namespace)

	o.Workspace = launcher.DefaultWorkspace
	flags.AddWorkspaceFlag(cmd, &o.Workspace)

	cmd.Flags().BoolVar(&o.DisableTTY, "no-tty", false, "disable tty")
	cmd.Flags().DurationVar(&o.PodTimeout, "pod-timeout", launcher.DefaultPodTimeout, "timeout for pod to be ready and running")
	cmd.Flags().DurationVar(&o.HandshakeTimeout, "handshake-timeout", v1alpha1.DefaultHandshakeTimeout, "timeout waiting for handshake")

	cmd.Flags().DurationVar(&o.ReconcileTimeout, "reconcile-timeout", launcher.DefaultReconcileTimeout, "timeout for resource to be reconciled")

	return cmd
}

func lookupEnvFile(cmd *cobra.Command, path string) (*env.Env, error) {
	etokenv, err := env.Read(path)
	if err != nil {
		if os.IsNotExist(err) {
			// A missing env file is OK
			return nil, nil
		}
		return nil, err
	}
	return etokenv, nil
}
