package podhandler

import (
	"context"
	"io"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/kubectl/pkg/cmd/attach"
	"k8s.io/kubectl/pkg/cmd/exec"
	"k8s.io/kubectl/pkg/scheme"
)

type PodHandler struct{}

// TODO: unit test
func (h *PodHandler) Attach(cfg *rest.Config, pod *corev1.Pod, in io.Reader, out, errOut io.Writer) error {
	cfg.ContentConfig = rest.ContentConfig{
		NegotiatedSerializer: scheme.Codecs.WithoutConversion(),
		GroupVersion:         &schema.GroupVersion{Version: "v1"},
	}
	cfg.APIPath = "/api"

	opts := &attach.AttachOptions{
		StreamOptions: exec.StreamOptions{
			Namespace:     pod.GetNamespace(),
			PodName:       pod.GetName(),
			ContainerName: "runner",
			Stdin:         true,
			TTY:           true,
			Quiet:         true,
			IOStreams: genericclioptions.IOStreams{
				In:     in,
				Out:    out,
				ErrOut: errOut,
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

func (h *PodHandler) GetLogs(ctx context.Context, kc kubernetes.Interface, pod *corev1.Pod, container string) (io.ReadCloser, error) {
	opts := &corev1.PodLogOptions{Follow: true, Container: container}
	return kc.CoreV1().Pods(pod.GetNamespace()).GetLogs(pod.GetName(), opts).Stream(ctx)
}
