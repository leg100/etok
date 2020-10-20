package cmd

var generateCmd = NewCmd("generate").WithShortHelp("Generate deployment resources")

func init() {
	root.AddChild(generateCmd)
}
