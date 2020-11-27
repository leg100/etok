package launcher

import (
	"errors"
	"fmt"
	"time"

	"github.com/leg100/stok/api/stok.goalspike.com/v1alpha1"
	"github.com/leg100/stok/cmd/flags"
	cmdutil "github.com/leg100/stok/cmd/util"
	"github.com/leg100/stok/pkg/env"
	stokerrors "github.com/leg100/stok/pkg/errors"
	"github.com/leg100/stok/util"
	"github.com/spf13/cobra"
)

var allCommands = []string{
	"apply",
	"destroy",
	"force-unlock",
	"get",
	"import",
	"init",
	"output",
	"plan",
	"refresh",
	"sh",
	"show",
	"state mv",
	"state pull",
	"state push",
	"state rm",
	"state show",
	"taint",
	"untaint",
	"validate",
}

// Add commands to root as subcommands (and subcommands' subcommands, and so on)
func AddCommandsToRoot(root *cobra.Command, opts *cmdutil.Options) {
	// Add terraform commands other than state
	for _, cmd := range nonStateCommands() {
		root.AddCommand(cmd.create(opts, &LauncherOptions{}))
	}

	// Add terraform state command
	state := &cobra.Command{
		Use:   "state",
		Short: "terraform state family of commands",
	}
	root.AddCommand(state)

	// ...and add terraform state sub commands to state command
	for _, cmd := range stateSubCommands() {
		state.AddCommand(cmd.create(opts, &LauncherOptions{}))
	}

	// Add shell command
	root.AddCommand(shellCommand().create(opts, &LauncherOptions{}))

	return
}

// Launcher command factory
type cmd struct {
	name, command, short string
	argsConverter        func([]string) []string
}

var TerraformCommands = append(nonStateCommandNames, "state")

var nonStateCommandNames = []string{
	"apply",
	"destroy",
	"force-unlock",
	"get",
	"import",
	"init",
	"output",
	"plan",
	"refresh",
	"show",
	"taint",
	"untaint",
	"validate",
}

// Terraform commands other than state
func nonStateCommands() (cmds []*cmd) {
	for _, name := range nonStateCommandNames {
		cmds = append(cmds, &cmd{
			name:    name,
			command: name,
			short:   fmt.Sprintf("Run terraform %s", name),
		})
	}
	return
}

// Terraform state sub commands
func stateSubCommands() (cmds []*cmd) {
	for _, name := range []string{"mv", "pull", "push", "rm", "show"} {
		cmds = append(cmds, &cmd{
			name:    name,
			short:   fmt.Sprintf("Run terraform state %s", name),
			command: fmt.Sprintf("state %s", name),
		})
	}
	return
}

// Sole non-terraform command: shell
func shellCommand() *cmd {
	return &cmd{
		name:          "sh",
		command:       "sh",
		short:         "Run shell commands in workspace",
		argsConverter: wrapShellArgs,
	}
}

// Spawn cobra command from launcher command factory
func (c *cmd) create(opts *cmdutil.Options, o *LauncherOptions) *cobra.Command {
	o.Options = opts
	o.Command = c.command

	// <namespace>/<workspace>
	var namespacedWorkspace string

	cmd := &cobra.Command{
		Use:   fmt.Sprintf("%s [flags] -- [%s args]", c.name, c.name),
		Short: c.short,
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			o.Namespace, o.Workspace, err = env.ValidateAndParse(namespacedWorkspace)
			if err != nil {
				return err
			}

			if c.argsConverter != nil {
				o.args = c.argsConverter(args)
			} else {
				o.args = args
			}

			o.RunName = fmt.Sprintf("run-%s", util.GenerateRandomString(5))

			o.Client, err = opts.Create(o.KubeContext)
			if err != nil {
				return err
			}

			if err := o.lookupEnvFile(cmd); err != nil {
				return err
			}

			err = o.Run(cmd.Context())
			if err != nil {
				// Cleanup resources upon error. An exit code error means the
				// runner ran successfully but the program it executed failed
				// with a non-zero exit code. In this case, resources are not
				// cleaned up.
				var exit stokerrors.ExitError
				if !errors.As(err, &exit) {
					if !o.DisableResourceCleanup {
						o.cleanup()
					}
				}
			}
			return err
		},
	}

	flags.AddPathFlag(cmd, &o.Path)
	flags.AddKubeContextFlag(cmd, &o.KubeContext)
	flags.AddDisableResourceCleanupFlag(cmd, &o.DisableResourceCleanup)

	cmd.Flags().BoolVar(&o.DisableTTY, "no-tty", false, "disable tty")
	cmd.Flags().DurationVar(&o.PodTimeout, "pod-timeout", time.Hour, "timeout for pod to be ready and running")
	cmd.Flags().DurationVar(&o.HandshakeTimeout, "handshake-timeout", v1alpha1.DefaultHandshakeTimeout, "timeout waiting for handshake")
	cmd.Flags().DurationVar(&o.EnqueueTimeout, "enqueue-timeout", 10*time.Second, "timeout waiting to be queued")
	cmd.Flags().StringVar(&namespacedWorkspace, "workspace", defaultWorkspace, "Stok workspace")

	return cmd
}
