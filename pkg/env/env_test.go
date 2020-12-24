package env

import (
	"testing"

	"github.com/leg100/etok/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnv(t *testing.T) {
	etokenv, err := New("default", "test-env")
	require.NoError(t, err)

	path := testutil.NewTempDir(t).Root()
	require.NoError(t, etokenv.Write(path))

	env, err := Read(path)
	require.NoError(t, err)

	assert.Equal(t, "default", env.Namespace)
	assert.Equal(t, "test-env", env.Workspace)
}
