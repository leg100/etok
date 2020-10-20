package cmd

var workspaceCmd = NewCmd("workspace").WithShortHelp("Stok workspace management")

func init() {
	root.AddChild(workspaceCmd)
}
