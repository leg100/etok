package controllers

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
			name:      "default",
			workspace: testobj.Workspace("default", "foo", testobj.WithTerraformVersion("0.12.17")),
			assertions: func(script string) {
				assert.Equal(t, `set -eo pipefail

echo Requested terraform version is 0.12.17

current_version=$(terraform version -json | jq '.terraform_version' -r)
echo Current terraform version is $current_version

if [[ 0.12.17 == $current_version ]]; then
  echo Skipping terraform installation
  exit 0
fi

echo
echo Downloading terraform 0.12.17...
curl -LOf# https://releases.hashicorp.com/terraform/0.12.17/terraform_0.12.17_linux_amd64.zip
echo
echo Downloading terraform 0.12.17 checksum...
curl -LOf# https://releases.hashicorp.com/terraform/0.12.17/terraform_0.12.17_SHA256SUMS

echo
echo Checking checksum...
sed -n '/terraform_0.12.17_linux_amd64.zip/p' terraform_0.12.17_SHA256SUMS | sha256sum -c

echo
echo Extracting terraform 0.12.17...
mkdir -p /terraform-bins
unzip terraform_0.12.17_linux_amd64.zip -d /terraform-bins
rm terraform_0.12.17_linux_amd64.zip
rm terraform_0.12.17_SHA256SUMS`, script)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := new(bytes.Buffer)
			require.NoError(t, generateScript(buf, tt.workspace))
			tt.assertions(buf.String())
		})
	}
}
