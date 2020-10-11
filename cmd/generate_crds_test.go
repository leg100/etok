package cmd

import (
	"bytes"
	"context"
	"io/ioutil"
	"os"
	"testing"

	"github.com/leg100/stok/pkg/app"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateCRDsFromLocal(t *testing.T) {
	// Command under test assumes it is invoked from parent directory (the root of the repo).
	previous, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir("../"))
	t.Cleanup(func() { os.Chdir(previous) })

	out := new(bytes.Buffer)
	opts, err := app.NewFakeOpts(out)
	require.NoError(t, err)

	assert.NoError(t, ParseArgs(context.Background(), []string{"generate", "crds", "--local"}, opts))

	crds, _ := ioutil.ReadFile(allCrdsPath)
	assert.Equal(t, string(crds), out.String())
}

// TODO: test default behaviour (retrieval of CRDs from remote URL)
