package controllers

import (
	"io"
	"text/template"

	"github.com/leg100/etok/api/etok.dev/v1alpha1"
)

var t = template.Must(template.New("workspace").Parse(newTemplate))

var newTemplate = `set -eo pipefail

echo Requested terraform version is {{ .Version }}

current_version=$(terraform version -json | jq '.terraform_version' -r)
echo Current terraform version is $current_version

if [[ {{ .Version }} == $current_version ]]; then
  echo Skipping terraform installation
  exit 0
fi

echo
echo Downloading terraform {{ .Version }}...
curl -LOf# https://releases.hashicorp.com/terraform/{{ .Version }}/terraform_{{ .Version }}_linux_amd64.zip
echo
echo Downloading terraform {{ .Version }} checksum...
curl -LOf# https://releases.hashicorp.com/terraform/{{ .Version }}/terraform_{{ .Version }}_SHA256SUMS

echo
echo Checking checksum...
sed -n '/terraform_{{ .Version }}_linux_amd64.zip/p' terraform_{{ .Version }}_SHA256SUMS | sha256sum -c

echo
echo Extracting terraform {{ .Version }}...
mkdir -p {{ .BinPath }}
unzip terraform_{{ .Version }}_linux_amd64.zip -d {{ .BinPath }}
rm terraform_{{ .Version }}_linux_amd64.zip
rm terraform_{{ .Version }}_SHA256SUMS`

func generateScript(out io.Writer, ws *v1alpha1.Workspace) error {
	return t.Execute(out, struct {
		Version                string
		BinPath                string
		TerraformWorkspaceName string
	}{
		Version:                ws.Spec.TerraformVersion,
		BinPath:                binMountPath,
		TerraformWorkspaceName: ws.TerraformName(),
	})
}
