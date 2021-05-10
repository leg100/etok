package launcher

import (
	cmdutil "github.com/leg100/etok/cmd/util"
	"github.com/leg100/etok/pkg/commands"
	"github.com/spf13/cobra"
)

// AddToRoot builds the cobra command tree of etok run commands ("plan",
// "apply", etc)
func AddToRoot(root *cobra.Command, f *cmdutil.Factory) {
	var added = make(map[string]*cobra.Command)
	for _, cmd := range commands.All {
		var add *cobra.Command
		parent := root
		if cmd.Parent() != "" {
			parent = added[cmd.Parent()]
		}
		if cmd.Unrunnable {
			add = &cobra.Command{
				Use:   cmd.Path,
				Short: cmd.GetShortDesc(),
			}
		} else {
			add = launcherCommand(f, &launcherOptions{command: cmd})
		}
		added[cmd.Path] = add
		parent.AddCommand(add)
	}
}
