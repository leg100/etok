package launcher

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/leg100/etok/api/etok.dev/v1alpha1"
	"github.com/leg100/etok/cmd/flags"
	cmdutil "github.com/leg100/etok/cmd/util"
	etokerrors "github.com/leg100/etok/pkg/errors"
	"github.com/leg100/etok/pkg/util"
	"github.com/spf13/cobra"
)

type runCommand struct {
	name, short, long string
	argsConverter     func([]string) []string
}

type runCommands []runCommand

var Cmds = runCommands{
	{
		name:  "apply",
		short: "Run terraform apply",
	},
	{
		name:  "destroy",
		short: "Run terraform destroy",
	},
	{
		name:  "plan",
		short: "Run terraform plan",
	},
	{
		name:  "sh",
		short: "Open shell session",
		argsConverter: func(args []string) []string {
			// Wrap shell args into a single command string
			if len(args) > 0 {
				return []string{"-c", strings.Join(args, " ")}
			} else {
				return []string{}
			}
		},
	},
}

func (rc runCommand) cobraCommand(opts *cmdutil.Options, o *launcherOptions) *cobra.Command {
	o.Options = opts
	o.command = rc.name

	cmd := &cobra.Command{
		Use:   fmt.Sprintf("%s [flags] -- [%s args]", rc.name, rc.name),
		Short: rc.short,
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			if rc.argsConverter != nil {
				o.args = rc.argsConverter(args)
			} else {
				o.args = args
			}

			// Tests override run name
			if o.runName == "" {
				o.runName = fmt.Sprintf("run-%s", util.GenerateRandomString(5))
			}

			o.Client, err = opts.Create(o.kubeContext)
			if err != nil {
				return err
			}

			if err := o.lookupEnvFile(cmd); err != nil {
				return err
			}

			err = o.run(cmd.Context())
			if err != nil {
				// Cleanup resources upon error. An exit code error means the
				// runner ran successfully but the program it executed failed
				// with a non-zero exit code. In this case, resources are not
				// cleaned up.
				var exit etokerrors.ExitError
				if !errors.As(err, &exit) {
					if !o.disableResourceCleanup {
						o.cleanup()
					}
				}
			}
			return err
		},
	}

	flags.AddPathFlag(cmd, &o.path)
	flags.AddNamespaceFlag(cmd, &o.namespace)
	flags.AddKubeContextFlag(cmd, &o.kubeContext)
	flags.AddDisableResourceCleanupFlag(cmd, &o.disableResourceCleanup)

	cmd.Flags().BoolVar(&o.disableTTY, "no-tty", false, "disable tty")
	cmd.Flags().DurationVar(&o.podTimeout, "pod-timeout", time.Hour, "timeout for pod to be ready and running")
	cmd.Flags().DurationVar(&o.handshakeTimeout, "handshake-timeout", v1alpha1.DefaultHandshakeTimeout, "timeout waiting for handshake")
	cmd.Flags().DurationVar(&o.enqueueTimeout, "enqueue-timeout", 10*time.Second, "timeout waiting to be queued")
	cmd.Flags().StringVar(&o.workspace, "workspace", defaultWorkspace, "etok workspace")

	cmd.Flags().DurationVar(&o.reconcileTimeout, "reconcile-timeout", defaultReconcileTimeout, "timeout for resource to be reconciled")

	return cmd
}

func (runCmds runCommands) CobraCommands(opts *cmdutil.Options) (cobraCmds []*cobra.Command) {
	for _, rc := range runCmds {
		cobraCmds = append(cobraCmds, rc.cobraCommand(opts, &launcherOptions{}))
	}
	return
}

func (runCmds runCommands) GetNames() (names []string) {
	for _, rc := range runCmds {
		names = append(names, rc.name)
	}
	return
}
