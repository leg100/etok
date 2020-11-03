package launcher

import (
	"fmt"
	"time"

	"github.com/leg100/stok/cmd/flags"
	"github.com/leg100/stok/pkg/app"
	"github.com/leg100/stok/util"
	"github.com/spf13/cobra"
)

// Add commands to root as subcommands (and subcommands' subcommands, and so on)
func AddCommandsToRoot(root *cobra.Command, opts *app.Options) {
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
func (c *cmd) create(opts *app.Options, o *LauncherOptions) *cobra.Command {
	o.Options = opts
	o.Command = c.command

	cmd := &cobra.Command{
		Use:   fmt.Sprintf("%s [flags] -- [%s args]", c.name, c.name),
		Short: c.short,
		RunE: func(cmd *cobra.Command, args []string) (err error) {
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
	flags.AddNamespaceFlag(cmd, &o.Namespace)
	flags.AddKubeContextFlag(cmd, &o.KubeContext)

	cmd.Flags().DurationVar(&o.TimeoutPod, "timeout-pod", time.Minute, "timeout for pod to be ready and running")
	cmd.Flags().DurationVar(&o.TimeoutClient, "timeout-client", defaultTimeoutClient, "timeout for client to signal readiness")
	cmd.Flags().DurationVar(&o.TimeoutQueue, "timeout-queue", time.Hour, "timeout waiting in workspace queue")
	cmd.Flags().DurationVar(&o.TimeoutEnqueue, "timeout-enqueue", 10*time.Second, "timeout waiting to be queued")
	cmd.Flags().StringVar(&o.Workspace, "workspace", defaultWorkspace, "Stok workspace")

	return cmd
}
