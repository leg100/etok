package launchers

import (
	cmdutil "github.com/leg100/etok/cmd/util"
	"github.com/leg100/etok/pkg/launcher"
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
		root.AddCommand(runCommand(f, &launcher.LauncherOptions{Command: cmd}))
	}

	// Terraform providers command
	providers := runCommand(f, &launcher.LauncherOptions{Command: "providers"})
	root.AddCommand(providers)

	// Terraform providers lock command
	providers.AddCommand(runCommand(f, &launcher.LauncherOptions{Command: "providers lock"}))

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
		state.AddCommand(runCommand(f, &launcher.LauncherOptions{Command: "state " + stateSubCmd}))
	}

	// Shell command
	shell := runCommand(f, &launcher.LauncherOptions{Command: "sh"})
	shell.Short = "Run shell session in workspace"
	root.AddCommand(shell)
}
