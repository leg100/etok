package command

import (
	"bytes"
	"strings"
	"text/template"

	v1alpha1 "github.com/leg100/stok/pkg/apis/stok/v1alpha1"
)

func generateScript(cr *v1alpha1.Command) (string, error) {
	script := `#Extract workspace tarball
tar zxf /tarball/{{ .Spec.ConfigMapKey }}

# wait for both the client to be ready and
# for the command to be front of the workspace queue
kubectl wait --for=condition=WorkspaceReady --timeout=-1s command/{{.Name}} > /dev/null
kubectl wait --for=condition=ClientReady --timeout=-1s command/{{.Name}} > /dev/null

# run stok command
{{join .Spec.Command " "}}{{ if gt (len .Spec.Args) 0 }} {{join .Spec.Args " " }}{{ end }}

`

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
