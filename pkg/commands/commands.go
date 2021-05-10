package commands

import (
	"strings"
)

// All supported etok run commands. Ensure parent command comes before any of
// its child commands, e.g. "state" before "state mv".
var All = []Command{
	{
		Path:      "apply",
		Queueable: true,
	},
	{
		Path: "console",
	},
	{
		Path:      "destroy",
		Queueable: true,
	},
	{
		Path:      "force-unlock",
		Queueable: true,
	},
	{
		Path: "get",
	},
	{
		Path: "graph",
	},
	{
		Path:      "import",
		Queueable: true,
	},
	{
		Path:            "init",
		Queueable:       true,
		UpdatesLockFile: true,
	},
	{
		Path: "output",
	},
	{
		Path: "plan",
	},
	{
		Path:      "refresh",
		Queueable: true,
	},
	{
		Path: "providers",
	},
	{
		Path:            "providers lock",
		UpdatesLockFile: true,
	},
	{
		Path: "show",
	},
	{
		Path:       "state",
		desc:       "Terraform state management",
		Unrunnable: true,
	},
	{
		Path: "state list",
	},
	{
		Path:      "state mv",
		Queueable: true,
	},
	{
		Path: "state pull",
	},
	{
		Path:      "state push",
		Queueable: true,
	},
	{
		Path:      "state replace-provider",
		Queueable: true,
	},
	{
		Path:      "state rm",
		Queueable: true,
	},
	{
		Path: "state show",
	},
	{
		Path:      "sh",
		desc:      "Run shell session in workspace",
		Queueable: true,
	},
	{
		Path:      "taint",
		Queueable: true,
	},
	{
		Path:      "untaint",
		Queueable: true,
	},
	{
		Path: "validate",
	},
}

func IsQueueable(cmd string) bool {
	for _, c := range All {
		if c.Path == cmd && c.Queueable {
			return true
		}
	}
	return false
}

func UpdatesLockFile(cmd string) bool {
	for _, c := range All {
		if c.Path == cmd && c.UpdatesLockFile {
			return true
		}
	}
	return false
}

// An etok run command
type Command struct {
	// The fully qualified command path, e.g. "state mv"
	Path string
	// Commands that are enqueued onto a workspace queue.
	Queueable bool
	// Commands that update the lock file (.terraform.lock.hcl)
	UpdatesLockFile bool
	// desc is a short description for the cobra command
	desc string
	// An Unrunnable command is not actually a supported etok run command but is
	// part of the cobra command tree, e.g. "state"
	Unrunnable bool
}

func (c *Command) String() string {
	return c.Path
}

// Extract parent part of command, if it has one.
func (c *Command) Parent() string {
	if parts := strings.Split(c.Path, " "); len(parts) > 1 {
		return parts[0]
	}
	return ""
}

// Extract child part of command.
func (c *Command) Child() string {
	if parts := strings.Split(c.Path, " "); len(parts) > 1 {
		return parts[1]
	}
	return c.Path
}

func (c *Command) GetShortDesc() string {
	if c.desc == "" {
		return c.defaultShortDesc()
	}
	return c.desc
}

func (c *Command) defaultShortDesc() string {
	return "Run terraform " + c.Path
}
