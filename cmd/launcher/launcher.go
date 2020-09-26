package launcher

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/leg100/stok/api/run"
	"github.com/leg100/stok/pkg/env"
	launchermod "github.com/leg100/stok/pkg/launcher"
	"github.com/leg100/stok/util/slice"
	"github.com/spf13/cobra"
)

func NewLauncherCmds(root *cobra.Command, args []string) []*cobra.Command {
	stokargs, tfargs := parseTerraformArgs(args)
	root.SetArgs(stokargs)

	var cmds []*cobra.Command

	for _, tfcmd := range run.TerraformCommands {
		launcher := &launchermod.Launcher{Command: tfcmd, Args: tfargs}
		if tfcmd == "sh" {
			// Wrap shell args into a single command string
			launcher.Args = wrapShellArgs(tfargs)
		} else {
			launcher.Args = tfargs
		}

		cmd := &cobra.Command{
			Use: tfcmd,
			RunE: func(cmd *cobra.Command, a []string) error {
				// If either namespace or workspace has not been set by user, then try to load them
				// from an env file
				stokenv, err := env.ReadStokEnv(launcher.Path)
				if err != nil {
					if !os.IsNotExist(err) {
						// It's ok for an environment file to not exist, but not any other error
						return err
					}
				} else {
					// Env file found, use namespace/workspace if user has not overridden them
					if !cmd.Flags().Changed("namespace") {
						launcher.Namespace = stokenv.Namespace()
					}
					if !cmd.Flags().Changed("workspace") {
						launcher.Workspace = stokenv.Workspace()
					}
				}

				debug, err := cmd.InheritedFlags().GetBool("debug")
				if err != nil {
					return err
				}
				launcher.Debug = debug

				launcher.Name = launchermod.GenerateName()

				return launcher.Run(cmd.Context())
			},
		}

		if tfcmd == "sh" {
			cmd.Short = "Run shell"
		} else {
			cmd.Short = fmt.Sprintf("Run terraform %s", tfcmd)
		}

		cmd.Flags().StringVar(&launcher.Path, "path", ".", "terraform config path")
		cmd.Flags().DurationVar(&launcher.TimeoutPod, "timeout-pod", time.Minute, "timeout for pod to be ready and running")
		cmd.Flags().DurationVar(&launcher.TimeoutClient, "timeout-client", 10*time.Second, "timeout for client to signal readiness")
		cmd.Flags().DurationVar(&launcher.TimeoutQueue, "timeout-queue", time.Hour, "timeout waiting in workspace queue")
		// TODO: rename to timeout-pending (enqueue is too similar sounding to queue)
		cmd.Flags().DurationVar(&launcher.TimeoutEnqueue, "timeout-enqueue", 10*time.Second, "timeout waiting to be queued")

		cmd.Flags().StringVar(&launcher.Workspace, "workspace", "default", "Workspace name")

		// Add flags registered by imported packages (controller-runtime)
		cmd.Flags().AddGoFlagSet(flag.CommandLine)

		cmds = append(cmds, cmd)
	}

	return cmds
}

func isLauncherCmd(args []string) bool {
	return len(args) > 0 && slice.ContainsString(run.TerraformCommands, args[0])
}

// Parse and return
// stok-specific args (those before '--'), and
// terraform-specific args (those after '--')
func parseTerraformArgs(args []string) (stokargs []string, tfargs []string) {
	if isLauncherCmd(args) {
		// Parse args after '--' only
		if i := slice.StringIndex(args, "--"); i > -1 {
			return append([]string{args[0]}, args[i+1:]...), args[1:i]
		} else {
			// No stok specific args
			return []string{args[0]}, args[1:]
		}
	} else {
		// Not a launcher command
		return args, []string{}
	}
}

// Wrap shell args into a single command string
func wrapShellArgs(args []string) []string {
	if len(args) > 0 {
		return []string{"-c", strings.Join(args, " ")}
	} else {
		return []string{}
	}
}
