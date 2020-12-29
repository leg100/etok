package launcher

import (
	"bytes"
	"context"
	"testing"

	"github.com/leg100/etok/cmd/runner"
	"github.com/stretchr/testify/assert"
)

func TestFmtCmd(t *testing.T) {
	out := new(bytes.Buffer)

	cmd := FmtCmd(&runner.FakeExecutorEchoArgs{Out: out})

	assert.NoError(t, cmd.ExecuteContext(context.Background()))
	assert.Equal(t, "[terraform fmt]", out.String())
}
