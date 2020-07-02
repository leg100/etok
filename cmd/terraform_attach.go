package cmd

import (
	"os"
	"strings"
	"time"

	"github.com/apex/log"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/rest"
	"k8s.io/kubectl/pkg/cmd/attach"
	"k8s.io/kubectl/pkg/cmd/exec"
	"k8s.io/kubectl/pkg/scheme"
)

// Attach to pod's TTY. Code borrowed from kubectl.
func (t *terraformCmd) handleAttachPod(config rest.Config, pod *corev1.Pod) error {
	config.ContentConfig = rest.ContentConfig{
		NegotiatedSerializer: scheme.Codecs.WithoutConversion(),
		GroupVersion:         &schema.GroupVersion{Version: "v1"},
	}
	config.APIPath = "/api"

	opts := &attach.AttachOptions{
		StreamOptions: exec.StreamOptions{
			Namespace: "default",
			PodName:   pod.GetName(),
			Stdin:     true,
			TTY:       true,
			Quiet:     true,
			IOStreams: genericclioptions.IOStreams{
				In:     os.Stdin,
				Out:    os.Stdout,
				ErrOut: attachErrOut{},
			},
		},
		Attach:        &attach.DefaultRemoteAttach{},
		AttachFunc:    attach.DefaultAttachFunc,
		GetPodTimeout: time.Second * 10,
	}

	opts.Config = &config
	opts.Pod = pod

	if err := opts.Run(); err != nil {
		return err
	}

	return nil
}

// ErrOut above wants an obj with Write method, so lets provide one that writes to our logger with
// warning level
type attachErrOut struct{}

func (_ attachErrOut) Write(in []byte) (int, error) {
	s := strings.TrimSpace(string(in))
	log.Warn(s)
	return 0, nil
}
