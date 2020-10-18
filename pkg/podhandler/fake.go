package podhandler

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type PodHandlerFake struct{}

func (h *PodHandlerFake) Attach(cfg *rest.Config, pod *corev1.Pod, out io.Writer) error {
	return fmt.Errorf("fake error")
}

func (h *PodHandlerFake) GetLogs(ctx context.Context, kc kubernetes.Interface, pod *corev1.Pod, container string) (io.ReadCloser, error) {
	return ioutil.NopCloser(bytes.NewBufferString("fake logs")), nil
}
