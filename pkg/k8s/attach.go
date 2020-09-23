package k8s

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/apex/log"
	"github.com/leg100/stok/pkg/k8s/config"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/rest"
	"k8s.io/kubectl/pkg/cmd/attach"
	"k8s.io/kubectl/pkg/cmd/exec"
	"k8s.io/kubectl/pkg/scheme"
)

var (
	Attach = attachToPod
)

// TODO: need to unit test the body of this method
func attachToPod(pod *corev1.Pod) error {
	cfg, err := config.GetConfig()
	if err != nil {
		return fmt.Errorf("getting client config for attaching to pod: %w", err)
	}
	cfg.ContentConfig = rest.ContentConfig{
		NegotiatedSerializer: scheme.Codecs.WithoutConversion(),
		GroupVersion:         &schema.GroupVersion{Version: "v1"},
	}
	cfg.APIPath = "/api"

	opts := &attach.AttachOptions{
		StreamOptions: exec.StreamOptions{
			// TODO: not sure how this has worked all this time for non-default namespaces?
			Namespace:     "default",
			PodName:       pod.GetName(),
			ContainerName: "runner",
			Stdin:         true,
			TTY:           true,
			Quiet:         true,
			IOStreams: genericclioptions.IOStreams{
				In:     os.Stdin,
				Out:    os.Stdout,
				ErrOut: attachErrOut{},
			},
		},
		Attach:     &attach.DefaultRemoteAttach{},
		AttachFunc: attach.DefaultAttachFunc,
		// TODO: parameterize
		GetPodTimeout: time.Second * 10,
	}

	opts.Config = cfg
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
