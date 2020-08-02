package maps

import (
	"sort"
	"testing"

	"gotest.tools/assert"
)

// Sort a map of string keys (and string values) by key
func SortStrings(m map[string]string) map[string]string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	mm := make(map[string]string, len(m))
	for _, k := range keys {
		mm[k] = m[k]
	}
	return mm
}

func TestSortStrings(t *testing.T) {
	m := SortStrings(map[string]string{
		"b": "2",
		"a": "1",
		"c": "3",
	})

	assert.Equal(t, map[string]string{
		"a": "1",
		"b": "2",
		"c": "3",
	}, m)
}
