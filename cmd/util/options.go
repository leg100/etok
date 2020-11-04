package util

import (
	"io"

	"github.com/leg100/stok/pkg/attacher"
	"github.com/leg100/stok/pkg/client"
	"github.com/leg100/stok/pkg/log"
	"github.com/leg100/stok/pkg/logstreamer"
	"k8s.io/apimachinery/pkg/runtime"
)

// TODO: move constants somewhere more appropriate
const (
	// MagicString is the string that the runner expects to receive prior to invoking its process.
	// Only take affects if RequireMagicString is true.
	MagicString = "magicstring"
	// The name of the container on the runner pod that invokes the runner
	ContainerName = "runner"
)

// Options pertaining to stok apps
type Options struct {
	// Deferred creation of clients
	client.ClientCreator

	// Function to attach to a pod's TTY
	attacher.AttachFunc

	// Function to get a pod's logs stream
	logstreamer.GetLogsFunc

	IOStreams

	Debug bool
}

// IOStreams provides the standard names for iostreams.  This is useful for embedding and for unit testing.
// Inconsistent and different names make it hard to read and review code
type IOStreams struct {
	// In think, os.Stdin
	In io.Reader
	// Out think, os.Stdout
	Out io.Writer
	// ErrOut think, os.Stderr
	ErrOut io.Writer
}

func NewOpts(out, errout io.Writer, in io.Reader) (*Options, error) {
	opts := &Options{
		GetLogsFunc:   logstreamer.GetLogs,
		AttachFunc:    attacher.Attach,
		ClientCreator: client.NewClientCreator(),
		IOStreams: IOStreams{
			Out:    out,
			ErrOut: errout,
			In:     in,
		},
	}
	// Set logger output device
	log.SetOut(opts.Out)
	return opts, nil
}

func NewFakeOpts(out io.Writer, objs ...runtime.Object) (*Options, error) {
	return &Options{
		GetLogsFunc:   logstreamer.FakeGetLogs,
		AttachFunc:    attacher.FakeAttach,
		ClientCreator: client.NewFakeClientCreator(objs...),
		IOStreams: IOStreams{
			Out: out,
		},
	}, nil
}
