package workspace

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeUrl(t *testing.T) {
	tests := []string{
		"https://github.com/leg100/etok.git",
		"git@github.com:leg100/etok.git",
		"ssh://git@github.com/leg100/etok.git",
	}

	for _, tt := range tests {
		assert.Equal(t, "https://github.com/leg100/etok.git", normalizeUrl(tt))
	}
}
