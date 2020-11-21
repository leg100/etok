package archive

import (
	"fmt"
	"io"
)

// ConfigMap/etcd only supports data payload of up to 1MB, which limits the size of the config that
// can be can be uploaded (after compression).
// https://github.com/kubernetes/kubernetes/issues/19781
const MaxConfigSize = 1024 * 1024

// MaxWriter implements Writer, wraps another Writer implementation, recording
// the number of bytes written, reporting an error when the total bytes written
// exceeds a given number. If max size is zero then no error will be reported.
type MaxWriter struct {
	tally, max int64
	w          io.Writer
}

func NewMaxWriter(w io.Writer, max int64) *MaxWriter {
	return &MaxWriter{w: w, max: max}
}

func (m *MaxWriter) Write(p []byte) (int, error) {
	m.tally += int64(len(p))
	if m.max != 0 {
		if m.tally > m.max {
			return 0, MaxSizeError(m.max)
		}
	}
	return m.w.Write(p)
}

type MaxSizeError int64

func (m MaxSizeError) Error() string {
	return fmt.Sprintf("max config size exceeded (%d bytes)", m)
}
