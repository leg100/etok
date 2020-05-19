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
	runtimeconfig "sigs.k8s.io/controller-runtime/pkg/client/config"
)

func (app *app) handleAttachPod(pod *corev1.Pod) error {
	// TODO: just horrible
	config := runtimeconfig.GetConfigOrDie()
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
				ErrOut: app,
			},
		},
		Attach:        &attach.DefaultRemoteAttach{},
		AttachFunc:    attach.DefaultAttachFunc,
		GetPodTimeout: time.Second * 10,
	}

	opts.Config = config
	opts.Pod = pod

	if err := opts.Run(); err != nil {
		return err
	}

	return nil
}

// permit app to be passed as a writer for the above handleAttachPod
// method
func (app *app) Write(in []byte) (int, error) {
	s := strings.TrimSpace(string(in))
	log.Warn(s)
	return 0, nil
}
