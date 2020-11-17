package archive

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModTree(t *testing.T) {
	mods, err := GetModules("../fixtures/modtree/root/mod")
	require.NoError(t, err)

	assert.Equal(t, []string{
		"root/mod",
		"root/mod/inner/mods/m2",
		"root/mod/inner/mods/m3",
		"outer/mods/m1",
	}, mods)
}
