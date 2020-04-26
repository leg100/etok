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
terraform output -json \
| jq -r 'to_entries
| map(select(.value.sensitive | not))
| map("\(.key)=\(.value.value)")
| .[]' \
> outputs.env

# persist outputs to configmap
kubectl create configmap {{.Name}}-state --from-env-file=outputs.env > /dev/null
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
