package github

import (
	"io"
	"text/template"
)

var (
	markdownFuncs = make(map[string]interface{})
	summary       *template.Template
)

func init() {
	markdownFuncs["quoted"] = func(s string) string { return "`" + s + "`" }

	summary = template.Must(template.New("summary").Funcs(markdownFuncs).Parse(`**Run:** {{ .RunName | quoted }}
`))
}

func generateSummary(out io.Writer, run *webhookCheckRun) error {
	return summary.Execute(out, struct {
		RunName string
	}{
		RunName: run.runName,
	})
}
