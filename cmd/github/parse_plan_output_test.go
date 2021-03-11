package github

import (
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParsePlanOutput(t *testing.T) {
	want := plan{
		adds:      2,
		changes:   0,
		deletions: 0,
	}

	output, err := ioutil.ReadFile("fixtures/plan.txt")
	require.NoError(t, err)

	plan, err := parsePlanOutput(string(output))
	require.NoError(t, err)
	assert.Equal(t, &want, plan)
}
