package commands

import (
	"fmt"
	"strings"
)

type Command struct {
	Name            string
	Local           bool
	NotRunnable     bool
	UpdatesLockFile bool
}

var Commands = []Command{
	{
		Name: "apply",
	},
	{
		Name: "console",
	},
	{
		Name: "destroy",
	},
	{
		Name:  "fmt",
		Local: true,
	},
	{
		Name: "force-unlock",
	},
	{
		Name: "get",
	},
	{
		Name: "graph",
	},
	{
		Name: "import",
	},
	{
		Name:            "init",
		UpdatesLockFile: true,
	},
	{
		Name: "output",
	},
	{
		Name: "plan",
	},
	{
		Name: "providers",
	},
	{
		Name:            "providers lock",
		UpdatesLockFile: true,
	},
	{
		Name: "refresh",
	},
	{
		Name: "sh",
	},
	{
		Name: "show",
	},
	{
		Name:        "state",
		NotRunnable: true,
	},
	{
		Name: "state mv",
	},
	{
		Name: "state pull",
	},
	{
		Name: "state push",
	},
	{
		Name: "state replace-provider",
	},
	{
		Name: "state rm",
	},
	{
		Name: "state show",
	},
	{
		Name: "taint",
	},
	{
		Name: "untaint",
	},
	{
		Name: "validate",
	},
}

func (c Command) ShortHelp() string {
	switch c.Name {
	case "sh":
		return "Open shell session on pod"
	default:
		// all other commands are terraform subcommands
		return fmt.Sprintf("Run terraform %s", c)
	}
}

func (c Command) String() string {
	return c.Name
}

func UpdatesLockFile(command string) bool {
	for _, c := range Commands {
		if c.Name == command && c.UpdatesLockFile {
			return true
		}
	}
	return false
}

// PrepareArgs manipulates the given args depending on the given command
func PrepareArgs(command string, args ...string) []string {
	switch command {
	case "sh":
		// Wrap shell args into a single command string
		if len(args) > 0 {
			return []string{"sh", "-c", strings.Join(args, " ")}
		} else {
			return []string{"sh"}
		}
	default:
		// all other commands are terraform subcommands
		return append([]string{"terraform"}, args...)
	}
}
