package cmd

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/leg100/stok/cmd/launcher"
	"github.com/leg100/stok/util/slice"
	"github.com/spf13/cobra"
)

type CommandGroup struct {
	Heading    string
	Commands   []*cobra.Command
	Summarized bool
}

func (g *CommandGroup) String() (s string) {
	cmds := []string{g.Heading}

	if g.Summarized {
		var b strings.Builder
		b.WriteString(g.Heading + "\n")
		w := tabwriter.NewWriter(&b, 5, 8, 1, '\t', 0)
		// Tabulated list of commands, three on each row
		for _, row := range slice.EqualChunkStrings(g.Names(), 3) {
			fmt.Fprintln(w, "  "+strings.Join(row, "\t"))
		}
		w.Flush()
		return b.String()
	}
	for _, cmd := range g.Commands {
		if cmd.IsAvailableCommand() {
			cmds = append(cmds, "  "+rpad(cmd.Name(), cmd.NamePadding())+" "+cmd.Short)
		}
	}
	return strings.Join(cmds, "\n")
}

func (g *CommandGroup) Names() (names []string) {
	for _, c := range g.Commands {
		names = append(names, c.Name())
	}
	return names
}

type CommandGroups []CommandGroup

func (grps CommandGroups) String() string {
	output := []string{}
	for _, g := range grps {
		output = append(output, g.String())
	}
	return strings.Join(output, "\n")
}

func CompileCommandGroups(cmd *cobra.Command) CommandGroups {
	if cmd == cmd.Root() {
		var tfCmds, stokCmds []*cobra.Command
		for _, c := range cmd.Commands() {
			if isTerraformCommand(c.Name()) {
				tfCmds = append(tfCmds, c)
			} else {
				stokCmds = append(stokCmds, c)
			}
		}
		return CommandGroups{
			{
				Heading:    "Terraform Commands:",
				Commands:   tfCmds,
				Summarized: true,
			},
			{
				Heading:  "Stok Commands:",
				Commands: stokCmds,
			},
		}
	}

	return CommandGroups{
		{
			Heading:  "Available Commands:",
			Commands: cmd.Commands(),
		},
	}
}

// All commands bar sh
func isTerraformCommand(name string) bool {
	return slice.ContainsString(
		append(launcher.TerraformCommands),
		name,
	)
}
