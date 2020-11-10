package launcher

import (
	"fmt"
	"time"

	"github.com/leg100/stok/api/stok.goalspike.com/v1alpha1"
	"github.com/leg100/stok/cmd/flags"
	cmdutil "github.com/leg100/stok/cmd/util"
	"github.com/leg100/stok/pkg/env"
	"github.com/leg100/stok/util"
	"github.com/spf13/cobra"
)

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

			return o.Run(cmd.Context())
		},
	}

	flags.AddPathFlag(cmd, &o.Path)
	flags.AddKubeContextFlag(cmd, &o.KubeContext)

	cmd.Flags().BoolVar(&o.DisableTTY, "no-tty", false, "disable tty")
	cmd.Flags().DurationVar(&o.TimeoutPod, "timeout-pod", time.Minute, "timeout for pod to be ready and running")
	cmd.Flags().DurationVar(&o.HandshakeTimeout, "handshake-timeout", v1alpha1.DefaultHandshakeTimeout, "Timeout waiting for handshake")
	cmd.Flags().DurationVar(&o.TimeoutQueue, "timeout-queue", time.Hour, "timeout waiting in workspace queue")
	cmd.Flags().DurationVar(&o.TimeoutEnqueue, "timeout-enqueue", 10*time.Second, "timeout waiting to be queued")
	cmd.Flags().StringVar(&namespacedWorkspace, "workspace", defaultWorkspace, "Stok workspace")

	return cmd
}
