package output

import (
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckRunOutput(t *testing.T) {
	got, err := ioutil.ReadFile("fixtures/got.txt")
	require.NoError(t, err)

	t.Run("within maximum size", func(t *testing.T) {
		o := newCheckRunOutput(false)

		_, err = o.Write(got)
		require.NoError(t, err)

		want, err := ioutil.ReadFile("fixtures/want.md")
		require.NoError(t, err)

		assert.Equal(t, string(want), o.output())
	})

	t.Run("exceeds maximum size", func(t *testing.T) {
		o := newCheckRunOutput(false)

		// Default is 64k but we'll set to an artificially low number so that we
		// can easily test this maximum being breached
		o.maxFieldSize = 1000

		_, err = o.Write(got)
		require.NoError(t, err)

		want, err := ioutil.ReadFile("fixtures/want_truncated.md")
		require.NoError(t, err)

		assert.Equal(t, string(want), o.output())
	})

	t.Run("strip off refreshing lines", func(t *testing.T) {
		o := newCheckRunOutput(true)

		_, err = o.Write(got)
		require.NoError(t, err)

		want, err := ioutil.ReadFile("fixtures/want_without_refresh.md")
		require.NoError(t, err)

		assert.Equal(t, string(want), o.output())
	})
}
