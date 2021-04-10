package util

import (
	"io"

	"github.com/leg100/etok/pkg/client"
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
	// Deferred creation of clients (k8s and etok clientsets)
	client.ClientCreator

	// Deferred creation of controller-runtime clients
	client.RuntimeClientCreator

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
		ClientCreator: client.NewFakeClientCreator(objs...),
		IOStreams: IOStreams{
			Out: out,
		},
	}
}
