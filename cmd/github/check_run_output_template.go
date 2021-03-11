package github

import (
	"io"
	"text/template"
)

var (
	markdownFuncs          = make(map[string]interface{})
	checkRunOutputTemplate *template.Template
)

func init() {
	markdownFuncs["quoted"] = func(s string) string { return "`" + s + "`" }
	markdownFuncs["textBlock"] = func(s string) string { return "```text\n" + s + "```" }

	checkRunOutputTemplate = template.Must(template.New("summary").Funcs(markdownFuncs).Parse(`
{{ if ne .RunOutput "" }}
{{ .RunOutput | textBlock }}
{{ end }}
{{ if ne .ErrOutput "" }}
{{ .ErrOutput | textBlock }}
{{ end }}
`))
}

func generateCheckRunOutput(out io.Writer, runOutput, errOutput string) error {
	return checkRunOutputTemplate.Execute(out, struct {
		RunOutput string
		ErrOutput string
	}{
		RunOutput: runOutput,
		ErrOutput: errOutput,
	})
}
