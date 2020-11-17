package modwalker

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModTree(t *testing.T) {
	mods, err := Walk("../fixtures/modtree/root/mod")
	require.NoError(t, err)

	assert.Equal(t, []string{
		"../fixtures/modtree/outer/mods/m1",
		"../fixtures/modtree/root/mod/inner/mods/m2",
		"../fixtures/modtree/root/mod/inner/mods/m3",
	}, mods)
}
