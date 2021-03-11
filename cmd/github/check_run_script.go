package github

import (
	"io"
	"text/template"
)

var checkRunScriptTemplate = template.Must(template.New("check_run_script").Parse(`set -e

terraform init -no-color -input=false

{{ if eq .Command "plan" }}
terraform plan -no-color -input=false -out={{ .PlanPath }}
{{ end }}

{{ if eq .Command "apply" }}
terraform apply -no-color -input=false {{ .PlanPath }}
{{ end }}
`))

func generateCheckRunScript(out io.Writer, planPath string, command string) error {
	return checkRunScriptTemplate.Execute(out, struct {
		Command  string
		PlanPath string
	}{
		Command:  command,
		PlanPath: planPath,
	})
}
