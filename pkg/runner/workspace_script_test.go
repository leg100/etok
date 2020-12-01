package runner

import (
	"bytes"
	"testing"

	"github.com/leg100/etok/api/etok.dev/v1alpha1"
	"github.com/leg100/etok/pkg/testobj"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkspaceScript(t *testing.T) {
	tests := []struct {
		name       string
		workspace  *v1alpha1.Workspace
		assertions func(string)
	}{
		{
			name:      "No download",
			workspace: testobj.Workspace("default", "foo"),
			assertions: func(script string) {
				assert.Equal(t, `set -eo pipefail

echo
echo Running terraform init...
terraform init -backend-config=backend.ini

echo
echo Running terraform workspace select default-foo...
set +e
terraform workspace select default-foo 2> /dev/null
exists=$?
set -e
if [[ $exists -ne 0 ]]; then
	echo
	echo Running terraform workspace new default-foo...
	terraform workspace new default-foo
fi`, script)
			},
		},
		{
			name:      "With download",
			workspace: testobj.Workspace("default", "foo", testobj.WithTerraformVersion("0.12.17")),
			assertions: func(script string) {
				assert.Equal(t, `set -eo pipefail

echo
echo Downloading terraform...
curl -LOf# https://releases.hashicorp.com/terraform/0.12.17/terraform_0.12.17_linux_amd64.zip
echo
echo Downloading terraform checksums...
curl -LOf# https://releases.hashicorp.com/terraform/0.12.17/terraform_0.12.17_SHA256SUMS

echo
echo Checking checksum...
sed -n '/terraform_0.12.17_linux_amd64.zip/p' terraform_0.12.17_SHA256SUMS | sha256sum -c

echo
echo Extracting terraform...
mkdir -p /terraform-bins
unzip terraform_0.12.17_linux_amd64.zip -d /terraform-bins
rm terraform_0.12.17_linux_amd64.zip
rm terraform_0.12.17_SHA256SUMS

echo
echo Running terraform init...
terraform init -backend-config=backend.ini

echo
echo Running terraform workspace select default-foo...
set +e
terraform workspace select default-foo 2> /dev/null
exists=$?
set -e
if [[ $exists -ne 0 ]]; then
	echo
	echo Running terraform workspace new default-foo...
	terraform workspace new default-foo
fi`, script)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := new(bytes.Buffer)
			require.NoError(t, generateWorkspaceScript(buf, tt.workspace))
			tt.assertions(buf.String())
		})
	}
}
