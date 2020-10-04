package cmd

import (
	"bytes"
	"context"
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGenerateCRDsFromLocal(t *testing.T) {
	// Command under test assumes it is invoked from parent directory (the root of the repo).
	previous, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir("../"))
	t.Cleanup(func() { os.Chdir(previous) })

	out := new(bytes.Buffer)
	code, err := ExecWithExitCode(context.Background(), []string{"generate", "crds", "--local"}, out, os.Stderr)

	require.NoError(t, err)
	require.Equal(t, 0, code)

	crds, _ := ioutil.ReadFile(allCrdsPath)
	require.Equal(t, string(crds), out.String())
}

// TODO: test default behaviour (retrieval of CRDs from remote URL)
