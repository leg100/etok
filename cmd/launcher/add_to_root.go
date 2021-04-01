package launcher

import (
	cmdutil "github.com/leg100/etok/cmd/util"
	"github.com/spf13/cobra"
)

func AddToRoot(root *cobra.Command, f *cmdutil.Factory) {
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
		root.AddCommand(launcherCommand(f, &launcherOptions{command: cmd}))
	}

	// Terraform providers command
	providers := launcherCommand(f, &launcherOptions{command: "providers"})
	root.AddCommand(providers)

	// Terraform providers lock command
	providers.AddCommand(launcherCommand(f, &launcherOptions{command: "providers lock"}))

	// Terraform state commands
	state := &cobra.Command{
		Use:   "state",
		Short: "Terraform state management",
	}
	root.AddCommand(state)

	for _, stateSubCmd := range []string{
		"list",
		"mv",
		"pull",
		"push",
		"replace-provider",
		"rm",
		"show",
	} {
		state.AddCommand(launcherCommand(f, &launcherOptions{command: "state " + stateSubCmd}))
	}

	// Shell command
	shell := launcherCommand(f, &launcherOptions{command: "sh"})
	shell.Short = "Run shell session in workspace"
	root.AddCommand(shell)
}
