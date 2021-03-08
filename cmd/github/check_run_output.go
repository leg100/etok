package output

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
)

const (
	// https://github.community/t/undocumented-65535-character-limit-on-requests/117564
	defaultMaxFieldSize = 65535
)

type checkRunOutput struct {
	out             []byte
	maxFieldSize    int
	stripRefreshing bool
}

func newCheckRunOutput(stripRefreshing bool) *checkRunOutput {
	return &checkRunOutput{
		maxFieldSize:    defaultMaxFieldSize,
		stripRefreshing: stripRefreshing,
	}
}

// Write logs to check run. Strips out certain content.
func (o *checkRunOutput) Write(p []byte) (int, error) {
	// Total bytes written
	var written int

	r := bufio.NewReader(bytes.NewBuffer(p))
	// Read segments of bytes delimited with a new line.
	for {
		line, err := r.ReadBytes('\n')
		written += len(line)
		if err == io.EOF {
			return written, nil
		}
		if err != nil {
			return written, err
		}

		if o.stripRefreshing && bytes.Contains(line, []byte(": Refreshing state... ")) {
			continue
		}

		if bytes.HasPrefix(line, []byte("  +")) || bytes.HasPrefix(line, []byte("  -")) || bytes.HasPrefix(line, []byte("  ~")) {
			// Trigger diff color highlighting by unindenting lines beginning
			// with +/-/~
			line = bytes.TrimLeft(line, " ")
		}

		o.out = append(o.out, line...)
	}
}

func (o *checkRunOutput) output() string {
	diffStart := "```diff\n"
	diffEnd := "\n```\n"

	if (len(diffStart) + len(o.out) + len(diffEnd)) <= o.maxFieldSize {
		return diffStart + string(bytes.TrimSpace(o.out)) + diffEnd
	}

	// Max bytes exceeded. Fetch new start position max bytes into output.
	start := len(o.out) - o.maxFieldSize

	// Account for diff headers
	start += len(diffStart)
	start += len(diffEnd)

	// Add message explaining reason. The number of bytes skipped is inaccurate:
	// it doesn't account for additional bytes skipped in order to accommodate
	// this message.
	exceeded := fmt.Sprintf("--- exceeded limit of %d bytes; skipping first %d bytes ---\n", o.maxFieldSize, start)

	// Adjust start position to account for message
	start += len(exceeded)

	// Ensure output does not start half way through a line. Remove bytes
	// leading up to and including the first new line character.
	if i := bytes.IndexByte(o.out[start:], '\n'); i > -1 {
		start += i + 1
	}

	// Trim off any remaining leading or trailing new lines
	trimmed := bytes.Trim(o.out[start:], "\n")

	return diffStart + exceeded + string(trimmed) + diffEnd
}
