package attacher

import (
	"io"
	"os"

	"golang.org/x/crypto/ssh/terminal"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/kubectl/pkg/util/term"

	"github.com/leg100/stok/pkg/log"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/kubectl/pkg/scheme"
)

type AttachFunc func(io.Writer, rest.Config, *corev1.Pod, *os.File, string, string) error

// Attach appropriates the behaviour of 'kubectl attach', adding a workaround for
// https://github.com/kubernetes/kubernetes/issues/27264. A 'handshake string' is sent, to inform the
// runner on the pod that the client has attached and, only then, will the runner invoke the
// process.
func Attach(out io.Writer, cfg rest.Config, pod *corev1.Pod, in *os.File, handshake, containerName string) error {
	cfg.ContentConfig = rest.ContentConfig{
		NegotiatedSerializer: scheme.Codecs.WithoutConversion(),
		GroupVersion:         &schema.GroupVersion{Version: "v1"},
	}
	cfg.APIPath = "/api"

	stdoutR, stdoutW := io.Pipe()
	stdinR, stdinW := io.Pipe()

	var oldState *terminal.State
	go func() {
		log.Info("Handshaking")
		// Blocks until read from stdinR
		_, err := stdinW.Write([]byte(handshake))
		if err != nil {
			panic(err)
		}
		// ...and can now proceed

		// Set stdin in raw mode.
		oldState, err = terminal.MakeRaw(int(in.Fd()))
		if err != nil {
			panic(err)
		}

		// Activate stdin
		go func() {
			io.Copy(stdinW, in)
		}()

		// Activate stdout
		io.Copy(out, stdoutR)
	}()

	defer func() {
		if oldState != nil {
			_ = terminal.Restore(int(in.Fd()), oldState)
		}
	}()

	t := term.TTY{
		Parent: nil,
		Out:    out,
		In:     in,
		Raw:    false,
	}

	var sizeQueue remotecommand.TerminalSizeQueue
	if size := t.GetSize(); size != nil {
		// fake resizing +1 and then back to normal so that attach-detach-reattach will result in the
		// screen being redrawn
		sizePlusOne := *size
		sizePlusOne.Width++
		sizePlusOne.Height++

		// this call spawns a goroutine to monitor/update the terminal size
		sizeQueue = t.MonitorSize(&sizePlusOne, size)
	}

	client, err := rest.RESTClientFor(&cfg)
	if err != nil {
		return err
	}

	exec, err := remotecommand.NewSPDYExecutor(&cfg, "POST", makeAttachRequest(client, pod, containerName).URL())
	if err != nil {
		return err
	}

	safeFn := func() error {
		return exec.Stream(remotecommand.StreamOptions{
			Stdin:             stdinR,
			Stdout:            stdoutW,
			Stderr:            os.Stderr, // not used when tty is true
			Tty:               true,
			TerminalSizeQueue: sizeQueue,
		})
	}
	if err := t.Safe(safeFn); err != nil {
		return err
	}
	return nil
}

func makeAttachRequest(client rest.Interface, pod *corev1.Pod, container string) *rest.Request {
	req := client.Post().
		Resource("pods").
		Name(pod.GetName()).
		Namespace(pod.GetNamespace()).
		SubResource("attach")

	return req.VersionedParams(&corev1.PodAttachOptions{
		Container: container,
		Stdin:     true,
		Stdout:    true,
		Stderr:    false,
		TTY:       true,
	}, scheme.ParameterCodec)
}
