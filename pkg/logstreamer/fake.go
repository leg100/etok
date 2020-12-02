package logstreamer

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
)

func FakeGetLogs(ctx context.Context, opts Options) (io.ReadCloser, error) {
	return ioutil.NopCloser(bytes.NewBufferString("fake logs")), nil
}
