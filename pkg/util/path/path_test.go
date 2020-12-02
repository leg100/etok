package path

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommonPrefix(t *testing.T) {
	paths := []string{
		"/home/user1/tmp/coverage/test",
		"/home/user1/tmp/covert/operator",
		"/home/user1/tmp/coven/members",
		"/home//user1/tmp/coventry",
		"/home/user1/././tmp/covertly/foo",
		"/home/bob/../user1/tmp/coved/bar",
	}

	prefix, err := CommonPrefix(paths)
	require.NoError(t, err)

	assert.Equal(t, "/home/user1/tmp", prefix)
}
