package runner

import (
	"io"
	"text/template"

	"github.com/leg100/stok/api/stok.goalspike.com/v1alpha1"
)

var t = template.Must(template.New("workspace").Parse(newWorkspaceTemplate))

var newWorkspaceTemplate = `set -eo pipefail{{ if ne .Version "" }}

echo
echo Downloading terraform...
curl -LOf# https://releases.hashicorp.com/terraform/{{ .Version }}/terraform_{{ .Version }}_linux_amd64.zip
echo
echo Downloading terraform checksums...
curl -LOf# https://releases.hashicorp.com/terraform/{{ .Version }}/terraform_{{ .Version }}_SHA256SUMS

echo
echo Checking checksum...
sed -n '/terraform_{{ .Version }}_linux_amd64.zip/p' terraform_{{ .Version }}_SHA256SUMS | sha256sum -c

echo
echo Extracting terraform...
mkdir -p {{ .BinPath }}
unzip terraform_{{ .Version }}_linux_amd64.zip -d {{ .BinPath }}
rm terraform_{{ .Version }}_linux_amd64.zip
rm terraform_{{ .Version }}_SHA256SUMS{{ end }}

echo
echo Running terraform init...
terraform init -backend-config={{ .BackendConfigFilename }}

echo
echo Running terraform workspace select {{ .TerraformWorkspaceName }}...
set +e
terraform workspace select {{ .TerraformWorkspaceName }} 2> /dev/null
exists=$?
set -e
if [[ $exists -ne 0 ]]; then
	echo
	echo Running terraform workspace new {{ .TerraformWorkspaceName }}...
	terraform workspace new {{ .TerraformWorkspaceName }}
fi`

func generateWorkspaceScript(out io.Writer, ws *v1alpha1.Workspace) error {
	return t.Execute(out, struct {
		Version                string
		BinPath                string
		TerraformWorkspaceName string
		BackendConfigFilename  string
	}{
		Version:                ws.Spec.TerraformVersion,
		BinPath:                terraformBinMountPath,
		TerraformWorkspaceName: ws.TerraformName(),
		BackendConfigFilename:  v1alpha1.BackendConfigFilename,
	})
}
