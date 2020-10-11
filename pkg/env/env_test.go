package env

import (
	"testing"

	"github.com/leg100/stok/testutil"
	"github.com/stretchr/testify/require"
	"github.com/tj/assert"
)

func TestStokEnv(t *testing.T) {
	path := testutil.NewTempDir(t).Root()
	require.NoError(t, NewStokEnv("default", "test-env").Write(path))

	env, err := ReadStokEnv(path)
	require.NoError(t, err)
	require.Equal(t, "default", env.Namespace())
	require.Equal(t, "test-env", env.Workspace())
}

func TestStokEnvValidate(t *testing.T) {
	assert.NoError(t, Validate("default/foo"))
	assert.NoError(t, Validate("foo"))
	assert.Error(t, Validate("foo$"))
	assert.Error(t, Validate("defaul/foo/"))
}