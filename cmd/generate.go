package cmd

var generateCmd = NewCmd("generate").WithShortHelp("Generate stok kubernetes resources")

func init() {
	root.AddChild(generateCmd)
}
