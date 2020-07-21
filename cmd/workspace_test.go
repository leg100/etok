package cmd

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEnvironmentFile(t *testing.T) {
	dest := createTempPath(t)
	require.NoError(t, writeEnvironmentFile(dest, "default", "test-env"))

	namespace, workspace, err := readEnvironmentFile(dest)
	require.NoError(t, err)
	require.Equal(t, "default", namespace)
	require.Equal(t, "test-env", workspace)
}
