package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"

	cmdutil "github.com/leg100/stok/cmd/util"
	"github.com/leg100/stok/pkg/signals"
	"golang.org/x/crypto/ssh/terminal"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/kubectl/pkg/scheme"
	"k8s.io/kubectl/pkg/util/term"
)

const (
	namespace     = "default"
	containerName = "sh"
	name          = "sh"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	signals.CatchCtrlC(cancel)

	go func() {
		<-ctx.Done()
		panic(fmt.Errorf("Interrupt received, exiting..."))
	}()

	opts, err := cmdutil.NewOpts(os.Stdout, os.Stderr, os.Stdin)
	if err != nil {
		log.Fatal(err)
	}

	client, err := opts.Create("")
	if err != nil {
		log.Fatal(err)
	}

	pod, err := client.PodsClient(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		log.Fatal(fmt.Errorf("getting pod %s/%s: %w", namespace, name, err))
	}

	cfg := client.Config

	attach(os.Stdout, *cfg, pod, os.Stdin, "magicstring", containerName)
}

func attach(out io.Writer, cfg rest.Config, pod *corev1.Pod, in *os.File, magicString, containerName string) {
	cfg.ContentConfig = rest.ContentConfig{
		NegotiatedSerializer: scheme.Codecs.WithoutConversion(),
		GroupVersion:         &schema.GroupVersion{Version: "v1"},
	}
	cfg.APIPath = "/api"

	stdoutR, stdoutW := io.Pipe()
	stdinR, stdinW := io.Pipe()

	var oldState *terminal.State
	go func() {
		fmt.Println("handshaking...")
		_, err := stdinW.Write([]byte("magicstring\n"))
		if err != nil {
			panic(err)
		}

		// Set stdin in raw mode.
		oldState, err = terminal.MakeRaw(int(os.Stdin.Fd()))
		if err != nil {
			panic(err)
		}

		// Activate stdin
		go func() { io.Copy(stdinW, os.Stdin) }()

		// Activate stdout
		io.Copy(os.Stdout, stdoutR)
	}()

	defer func() {
		if oldState != nil {
			if err := terminal.Restore(int(os.Stdin.Fd()), oldState); err != nil {
				log.Printf("tried to restore tty state: %v", err)
			} else {
				log.Printf("terminal state restored successfully\n")
			}
		}
	}()

	t := term.TTY{
		Parent: nil,
		Out:    os.Stdout,
		In:     os.Stdin,
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

	restClient, err := rest.RESTClientFor(&cfg)
	if err != nil {
		log.Fatal(err)
	}

	exec, err := remotecommand.NewSPDYExecutor(&cfg, "POST", makeAttachRequest(restClient, pod, containerName).URL())
	if err != nil {
		log.Fatal(err)
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
		panic(err)
	}
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
