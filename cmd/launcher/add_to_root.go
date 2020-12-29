package launcher

import (
	cmdutil "github.com/leg100/etok/cmd/util"
	"github.com/spf13/cobra"
)

func AddToRoot(root *cobra.Command, opts *cmdutil.Options) {
	// Terraform commands
	for _, cmd := range []string{
		"apply",
		"console",
		"destroy",
		"force-unlock",
		"get",
		"graph",
		"import",
		"init",
		"output",
		"plan",
		"refresh",
		"show",
		"taint",
		"untaint",
		"validate",
	} {
		root.AddCommand(LauncherCommand(opts, &launcherOptions{command: cmd}))
	}

	// Terraform providers command
	providers := LauncherCommand(opts, &launcherOptions{command: "providers"})
	root.AddCommand(providers)

	// Terraform providers lock command
	providers.AddCommand(LauncherCommand(opts, &launcherOptions{command: "providers lock"}))

	// Terraform state commands
	state := &cobra.Command{
		Use:   "state",
		Short: "Terraform state management",
	}
	root.AddCommand(state)

	for _, stateSubCmd := range []string{
		"mv",
		"pull",
		"push",
		"replace-provider",
		"rm",
		"show",
	} {
		state.AddCommand(LauncherCommand(opts, &launcherOptions{command: "state " + stateSubCmd}))
	}

	// Shell command
	shell := LauncherCommand(opts, &launcherOptions{command: "sh"})
	shell.Short = "Run shell session in workspace"
	root.AddCommand(shell)
}
