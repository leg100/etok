package podhandler

import (
	"context"
	"io"
	"os"
	"strings"
	"time"

	"github.com/leg100/stok/pkg/log"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/kubectl/pkg/cmd/attach"
	"k8s.io/kubectl/pkg/cmd/exec"
	"k8s.io/kubectl/pkg/scheme"
)

type PodHandler struct {}

// TODO: need to unit test the body of this method
func (h *PodHandler) Attach(cfg *rest.Config, pod *corev1.Pod) error {
	// TODO: deep copy cfg
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
		Config:        cfg,
		Pod:           pod,
	}

	return opts.Run()
}

// ErrOut above wants an obj with Write method, so lets provide one that writes to our logger with
// info level
type attachErrOut struct{}

func (_ attachErrOut) Write(in []byte) (int, error) {
	s := strings.TrimSpace(string(in))
	log.Info(s)
	return len(in), nil
}

func (h *PodHandler) GetLogs(ctx context.Context, kc kubernetes.Interface, pod *corev1.Pod, container string) (io.ReadCloser, error) {
	opts := &corev1.PodLogOptions{Follow: true, Container: container}
	return kc.CoreV1().Pods(pod.GetNamespace()).GetLogs(pod.GetName(), opts).Stream(ctx)
}
