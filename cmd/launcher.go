package cmd

import (
	"flag"
	"fmt"
	"strings"
	"time"

	"github.com/leg100/stok/api/run"
	"github.com/leg100/stok/cmd/flags"
	"github.com/leg100/stok/pkg/apps/launcher"
	"github.com/leg100/stok/pkg/options"
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
	return NewCmd(tfcmd).
		WithShortUsage(fmt.Sprintf("%s [flags] -- [command flags|args]", tfcmd)).
		WithShortHelp(launcherShortHelp(tfcmd)).
		WithFlags(
			flags.Path,
			func(fs *flag.FlagSet, opts *options.StokOptions) {
				fs.DurationVar(&opts.TimeoutPod, "timeout-pod", time.Minute, "timeout for pod to be ready and running")
				fs.DurationVar(&opts.TimeoutClient, "timeout-client", 10*time.Second, "timeout for client to signal readiness")
				fs.DurationVar(&opts.TimeoutQueue, "timeout-queue", time.Hour, "timeout waiting in workspace queue")
				fs.DurationVar(&opts.TimeoutEnqueue, "timeout-enqueue", 10*time.Second, "timeout waiting to be queued")

				fs.StringVar(&opts.StokEnv, "workspace", "default", "Stok workspace")
			},
		).
		WithOneArg().
		WithPreExec(func(fs *flag.FlagSet, opts *options.StokOptions) error {
			// Bring tfcmd into local lexical scope
			tfcmd := tfcmd

			if tfcmd == "sh" {
				// Wrap shell args into a single command string
				opts.Args = wrapShellArgs(opts.Args)
			}

			opts.Name = launcher.GenerateName()
			opts.Command = tfcmd

			if err := opts.SetWorkspace(fs); err != nil {
				return err
			}

			if err := opts.SetNamespace(fs); err != nil {
				return err
			}

			return nil
		}).
		WithApp(launcher.NewFromOptions)
}

func launcherShortHelp(tfcmd string) string {
	if tfcmd == "sh" {
		return "Run shell"
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
