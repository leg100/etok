package util

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGenerateRandomString(t *testing.T) {
	require.Regexp(t, "[a-z0-9]{5}", GenerateRandomString(5))
}
