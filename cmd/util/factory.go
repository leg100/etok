package util

import (
	"io"

	"github.com/leg100/etok/pkg/attacher"
	"github.com/leg100/etok/pkg/client"
	"github.com/leg100/etok/pkg/logstreamer"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
)

// TODO: move constants somewhere more appropriate
const (
	// HandshakeString is the string that the runner expects to receive via stdin prior to running.
	HandshakeString = "opensesame"
)

// Factory pertaining to etok apps
type Factory struct {
	// Deferred creation of clients
	client.ClientCreator

	// Deferred creation of controller-runtime clients
	client.RuntimeClientCreator

	// Function to attach to a pod's TTY
	attacher.AttachFunc

	// Function to get a pod's logs stream
	logstreamer.GetLogsFunc

	IOStreams

	Verbosity int
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

func NewFactory(out, errout io.Writer, in io.Reader) *Factory {
	f := &Factory{
		GetLogsFunc:          logstreamer.GetLogs,
		AttachFunc:           attacher.Attach,
		ClientCreator:        client.NewClientCreator(),
		RuntimeClientCreator: client.NewRuntimeClientCreator(),
		IOStreams: IOStreams{
			Out:    out,
			ErrOut: errout,
			In:     in,
		},
	}
	// Set logger output device
	klog.SetOutput(f.Out)
	return f
}

func NewFakeFactory(out io.Writer, objs ...runtime.Object) *Factory {
	return &Factory{
		GetLogsFunc:          logstreamer.FakeGetLogs,
		AttachFunc:           attacher.FakeAttach,
		ClientCreator:        client.NewFakeClientCreator(objs...),
		RuntimeClientCreator: client.NewRuntimeClientCreator(),
		IOStreams: IOStreams{
			Out: out,
		},
	}
}
