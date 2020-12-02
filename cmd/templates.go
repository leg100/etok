package cmd

import (
	"strings"
	"unicode"
)

const (
	// SectionAliases is the help template section that displays command aliases.
	SectionUsage = `Usage: {{if .Runnable}}{{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}{{.CommandPath}} [command]{{end}}

`

	// SectionAliases is the help template section that displays command aliases.
	SectionAliases = `{{if gt .Aliases 0}}Aliases:
{{.NameAndAliases}}

{{end}}`

	// SectionExamples is the help template section that displays command examples.
	SectionExamples = `{{if .HasExample}}Examples:
{{trimRight .Example}}

{{end}}`

	// SectionSubcommands is the help template section that displays the command's subcommands.
	SectionSubcommands = `{{if .HasAvailableSubCommands}}{{cmdGroupsString .}}

{{end}}`

	// SectionFlags is the help template section that displays the command's flags.
	SectionFlags = `{{ if .HasAvailableLocalFlags }}Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}

{{end}}`

	// SectionFlags is the help template section that displays the command's flags.
	SectionGlobalFlags = `{{ if .HasAvailableInheritedFlags }}Global Flags:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}

{{end}}`

	// SectionTipsHelp is the help template section that displays the '--help' hint.
	SectionTipsHelp = `{{if .HasAvailableSubCommands}}Use "{{.CommandPath}} [command] --help" for more information about a command.
{{end}}`
)

// MainHelpTemplate if the template for 'help' used by most commands.
func MainHelpTemplate() string {
	return `{{if or .Runnable .HasSubCommands}}{{.UsageString}}{{end}}`
}

// MainUsageTemplate if the template for 'usage' used by most commands.
func MainUsageTemplate() string {
	sections := []string{
		SectionUsage,
		SectionAliases,
		SectionExamples,
		SectionSubcommands,
		SectionFlags,
		SectionGlobalFlags,
		SectionTipsHelp,
	}
	return strings.TrimRightFunc(strings.Join(sections, ""), unicode.IsSpace)
}

func usageTemplate() string {
	return `Usage:{{if .Runnable}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [command]{{end}}{{if gt (len .Aliases) 0}}

Aliases:
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

Examples:
{{.Example}}{{end}}{{if .IsRoot }}

Terraform Commands:{{range .Commands}}{{if isTerraformCommand .Name}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{else}}

Etok Commands:{{range .Commands}}{{if and (not (isTerraformCommand .Name)) (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{end}}{{else}}{{ if .HasAvailableSubCommands}}

Available Commands:{{range .Commands}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

Global Flags:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasHelpSubCommands}}

Additional help topics:{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

Use "{{.CommandPath}} [command] --help" for more information about a command.{{end}}
`
}
