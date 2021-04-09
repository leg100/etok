package logstreamer

import (
	"bytes"
	"context"
	"io"
)

func FakeGetLogs(ctx context.Context, opts Options) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewBufferString("fake logs")), nil
}
