package slice

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSlice(t *testing.T) {
	assert.Equal(t, [][]string{{"a", "b"}, {"c", "d"}, {"e"}}, ChunkStrings([]string{"a", "b", "c", "d", "e"}, 2))
	assert.Equal(t, [][]string{{"a", "b"}, {"c", "d"}, {"e", ""}}, EqualChunkStrings([]string{"a", "b", "c", "d", "e"}, 2))
	//assert.Equal(t, [][]string{{"a", "b"}, {"c", "d"}}, ChunkStrings([]string{"a", "b", "c", "d"}, 2))
}
