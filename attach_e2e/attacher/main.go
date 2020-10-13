package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/fatih/color"
	"github.com/leg100/stok/pkg/app"
	"github.com/leg100/stok/pkg/podhandler"
	"github.com/leg100/stok/pkg/signals"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	namespace = "default"
	podname   = "default"
)

func init() {
	flag.StringVar(&podname, "name", "default", "")
	flag.StringVar(&namespace, "namespace", "default", "")
	flag.Parse()
}

// system-under-test: attach to a k8s pod
func main() {
	// Create context, and cancel if interrupt is received
	ctx, cancel := context.WithCancel(context.Background())
	signals.CatchCtrlC(cancel)

	opts, err := app.NewOpts()
	if err != nil {
		panic(err)
	}

	opts.CreateClients("")

	errch := make(chan error)
	ready := make(chan struct{})

	// Watch for pod events
	(&podMonitor{
		namespace: namespace,
		name:      podname,
		client:    opts.KubeClient(),
	}).monitor(ctx, errch, ready)

	// Get pod
	pod, err := opts.KubeClient().CoreV1().Pods(namespace).Get(ctx, podname, metav1.GetOptions{})
	if err != nil {
		exitWithError(err)
	}

	// Wait for pod to be ready
	select {
	case <-ready:
		// Attach to pod
		err = (&podhandler.PodHandler{}).Attach(opts.KubeConfig(), pod, os.Stdout)
		if err != nil {
			exitWithError(err)
		}
	case <-time.After(10 * time.Second):
		exitWithError(fmt.Errorf("timed out waiting for pod to be ready"))
	case <-ctx.Done():
		exitWithError(ctx.Err())
	case err := <-errch:
		exitWithError(err)
	}

	//fmt.Printf(unix.VEOF)
	//newState := *termios
	//newState.
	//unix.IoctlSetTermios(os.Stdout.Fd(), unix.TCSETS,
}

func exitWithError(err error) {
	fmt.Fprintf(os.Stderr, "%s %s\n", color.HiRedString("Error:"), err.Error())
	os.Exit(1)
}
