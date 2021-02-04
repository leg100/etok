package backup

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfig(t *testing.T) {
	cfg := NewConfig()
	assert.Equal(t, []string{"gcs", "s3"}, cfg.providers())
}
