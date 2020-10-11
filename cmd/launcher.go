package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/leg100/stok/api/run"
	"github.com/leg100/stok/cmd/flags"
	"github.com/leg100/stok/pkg/app"
	"github.com/leg100/stok/pkg/apps/launcher"
	"github.com/spf13/pflag"
)

func init() {
	for k, v := range run.TerraformCommandMap {
		if len(v) > 0 {
			// Terraform 'family' of commands, i.e. terraform show mv|rm|pull|push|show
			parent := NewCmd(k).WithShortHelp(fmt.Sprintf("terraform %s family of commands", k))
			root.AddChild(parent)
			for _, child := range v {
				parent.AddChild(launcherCmd(child))
			}
		} else {
			root.AddChild(launcherCmd(k))
		}
	}
}

func launcherCmd(tfcmd string) Builder {
	return NewCmd(fmt.Sprintf("%s [flags] -- [%s args]", tfcmd, tfcmd)).
		WithShortHelp(launcherShortHelp(tfcmd)).
		WithFlags(
			flags.Path,
			flags.Namespace,
			func(fs *pflag.FlagSet, opts *app.Options) {
				fs.DurationVar(&opts.TimeoutPod, "timeout-pod", time.Minute, "timeout for pod to be ready and running")
				fs.DurationVar(&opts.TimeoutClient, "timeout-client", 10*time.Second, "timeout for client to signal readiness")
				fs.DurationVar(&opts.TimeoutQueue, "timeout-queue", time.Hour, "timeout waiting in workspace queue")
				fs.DurationVar(&opts.TimeoutEnqueue, "timeout-enqueue", 10*time.Second, "timeout waiting to be queued")

				fs.StringVar(&opts.Workspace, "workspace", "default", "Stok workspace")
			},
		).
		WithPreExec(func(fs *pflag.FlagSet, opts *app.Options) error {
			// Bring tfcmd into local lexical scope
			tfcmd := tfcmd

			if tfcmd == "sh" {
				// Wrap shell args into a single command string
				opts.Args = wrapShellArgs(opts.Args)
			}

			opts.Command = tfcmd

			if err := opts.SetNamespaceAndWorkspaceFromEnv(fs); err != nil {
				return err
			}

			return nil
		}).
		WantsKubeClients().
		WithApp(launcher.NewFromOpts)
}

func launcherShortHelp(tfcmd string) string {
	if tfcmd == "sh" {
		return "Run shell commands in workspace"
	} else {
		return fmt.Sprintf("Run terraform %s", tfcmd)
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
