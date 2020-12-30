package labels

import (
	"testing"

	"gotest.tools/assert"
)

func TestLabels(t *testing.T) {
	assert.Equal(t, Label{Name: "foo", Value: "bar"}, NewLabel("foo", "bar"))
	assert.Equal(t, Label{Name: "foo", Value: "bar-with-spaces"}, NewLabel("foo", "bar with spaces"))
	assert.Equal(t, Label{Name: "foo-with-spaces", Value: "bar"}, NewLabel("foo with spaces", "bar"))
}
