package env

import (
	"testing"

	"github.com/leg100/etok/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEtokEnv(t *testing.T) {
	path := testutil.NewTempDir(t).Root()
	require.NoError(t, NewEtokEnv("default", "test-env").Write(path))

	env, err := ReadEtokEnv(path)
	require.NoError(t, err)

	assert.Equal(t, "default", env.Namespace())
	assert.Equal(t, "test-env", env.Workspace())
}

func TestEtokEnvValidate(t *testing.T) {
	assert.NoError(t, Validate("default/foo"))
	assert.Error(t, Validate("defaul/foo/"))
}
