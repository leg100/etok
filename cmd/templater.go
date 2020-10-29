package cmd

import (
	"fmt"
	"strings"
	"text/template"
	"unicode"

	"github.com/spf13/cobra"
	"k8s.io/kubectl/pkg/util/term"
)

type templater struct {
	UsageTemplate string
}

func (templater *templater) UsageFunc() func(*cobra.Command) error {
	return func(c *cobra.Command) error {
		t := template.New("usage")
		t.Funcs(templater.templateFuncs())
		template.Must(t.Parse(templater.UsageTemplate))
		out := term.NewResponsiveWriter(c.OutOrStderr())
		return t.Execute(out, c)
	}
}

func (t *templater) cmdGroupsString(c *cobra.Command) string {
	return CompileCommandGroups(c).String()
}

func (t *templater) templateFuncs() template.FuncMap {
	return template.FuncMap{
		"trim":                    strings.TrimSpace,
		"trimRight":               func(s string) string { return strings.TrimRightFunc(s, unicode.IsSpace) },
		"trimLeft":                func(s string) string { return strings.TrimLeftFunc(s, unicode.IsSpace) },
		"gt":                      cobra.Gt,
		"eq":                      cobra.Eq,
		"rpad":                    rpad,
		"cmdGroupsString":         t.cmdGroupsString,
		"trimTrailingWhitespaces": trimRightSpace,
	}
}

// rpad adds padding to the right of a string.
func rpad(s string, padding int) string {
	template := fmt.Sprintf("%%-%ds", padding)
	return fmt.Sprintf(template, s)
}

func trimRightSpace(s string) string {
	return strings.TrimRightFunc(s, unicode.IsSpace)
}
