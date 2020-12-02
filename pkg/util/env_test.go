package util

import (
	"testing"

	"github.com/leg100/etok/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnvironmentFile(t *testing.T) {
	dest := testutil.NewTempDir(t)
	require.NoError(t, WriteEnvironmentFile(dest.Root(), "default", "test-env"))

	namespace, workspace, err := ReadEnvironmentFile(dest.Root())
	require.NoError(t, err)

	assert.Equal(t, "default", namespace)
	assert.Equal(t, "test-env", workspace)
}
