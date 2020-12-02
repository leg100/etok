package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateRandomString(t *testing.T) {
	assert.Regexp(t, "[a-z0-9]{5}", GenerateRandomString(5))
}
