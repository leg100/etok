package cmd

import (
	"bytes"
	"io/ioutil"
	"os"
	"testing"

	"github.com/leg100/stok/pkg/k8s/fake"
	"github.com/stretchr/testify/require"
)

func TestGenerateCRDsFromLocal(t *testing.T) {
	out := new(bytes.Buffer)
	var cmd = newStokCmd(fake.NewFactory(), out, os.Stderr)

	// Command under test assumes it is invoked from parent directory (the root of the repo).
	previous, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir("../"))
	t.Cleanup(func() { os.Chdir(previous) })

	code, err := cmd.Execute([]string{
		"generate",
		"crds",
		"--local",
	})

	require.NoError(t, err)
	require.Equal(t, 0, code)

	crds, _ := ioutil.ReadFile(allCrdsPath)
	require.Equal(t, string(crds), out.String())
}

// TODO: test default behaviour (retrieval of CRDs from remote URL)
