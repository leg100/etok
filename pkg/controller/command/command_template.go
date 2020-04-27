package command

import (
	"bytes"
	"strings"
	"text/template"

	terraformv1alpha1 "github.com/leg100/stok/pkg/apis/terraform/v1alpha1"
)

func generateScript(cr *terraformv1alpha1.Command) (string, error) {
	script := `#Extract workspace tarball
tar zxf /tarball/{{ .Spec.ConfigMapKey }}

# wait for client to inform us they're streaming logs
kubectl wait --for=condition=ClientReady command/{{.Name}} > /dev/null

# run stok command
{{join .Spec.Command " "}}{{ if gt (len .Spec.Args) 0 }} {{join .Spec.Args " " }}{{ end }}

{{ if gt (len .Spec.Args) 0 }}{{ if eq (index .Spec.Args 0) "apply" }}# capture outputs if apply command was run
outputs=$(terraform output -json)

# filter out sensitive values and convert to flat JSON object
patch=$(echo $outputs \
| jq -r 'to_entries
| map(select(.value.sensitive | not))
| map({(.key): (.value.value | tostring)})
| .[]' \
| jq -cs add)

# persist outputs to configmap
kubectl patch configmap {{.Name}} -p  "{\"data\": $patch}" > /dev/null
{{ end }}{{ end }}`

	tmpl := template.New("script")
	tmpl = tmpl.Funcs(template.FuncMap{"join": strings.Join})
	tmpl, err := tmpl.Parse(script)
	if err != nil {
		return "", err
	}

	buf := new(bytes.Buffer)
	err = tmpl.Execute(buf, cr)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}
