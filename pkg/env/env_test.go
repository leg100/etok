package env

import (
	"testing"

	"github.com/leg100/stok/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStokEnv(t *testing.T) {
	path := testutil.NewTempDir(t).Root()
	require.NoError(t, NewStokEnv("default", "test-env").Write(path))

	env, err := ReadStokEnv(path)
	require.NoError(t, err)

	assert.Equal(t, "default", env.Namespace())
	assert.Equal(t, "test-env", env.Workspace())
}

func TestStokEnvValidate(t *testing.T) {
	assert.NoError(t, Validate("default/foo"))
	assert.Error(t, Validate("defaul/foo/"))
}
