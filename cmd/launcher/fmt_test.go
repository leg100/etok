package launcher

import (
	"bytes"
	"context"
	"testing"

	"github.com/leg100/etok/pkg/executor"
	"github.com/stretchr/testify/assert"
)

func TestFmtCmd(t *testing.T) {
	out := new(bytes.Buffer)

	cmd := FmtCmd(&executor.FakeExecutorEchoArgs{Out: out})

	assert.NoError(t, cmd.ExecuteContext(context.Background()))
	assert.Equal(t, "[terraform fmt]", out.String())
}
