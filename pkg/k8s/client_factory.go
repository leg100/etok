package k8s

import (
	"io"
	"os"
	"strings"
	"time"

	"github.com/apex/log"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/kubectl/pkg/cmd/attach"
	"k8s.io/kubectl/pkg/cmd/exec"
	"k8s.io/kubectl/pkg/scheme"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type FactoryInterface interface {
	NewClient(*rest.Config, *runtime.Scheme) (Client, error)
}

type Factory struct{}

var _ FactoryInterface = &Factory{}

type Client interface {
	runtimeclient.Client
	GetLogs(string, string, *corev1.PodLogOptions) (io.ReadCloser, error)
	Attach(*corev1.Pod) error
}

type client struct {
	runtimeclient.Client
	kc     kubernetes.Interface
	config *rest.Config
}

func (f *Factory) NewClient(config *rest.Config, s *runtime.Scheme) (Client, error) {
	rc, err := runtimeclient.New(config, runtimeclient.Options{Scheme: s})
	if err != nil {
		return nil, err
	}

	kc, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &client{Client: rc, kc: kc, config: config}, nil
}

func (c *client) GetLogs(namespace, name string, opts *corev1.PodLogOptions) (io.ReadCloser, error) {
	req := c.kc.CoreV1().Pods(namespace).GetLogs(name, opts)
	return req.Stream()
}

// TODO: need to unit test the body of this method
func (c *client) Attach(pod *corev1.Pod) error {
	c.config.ContentConfig = rest.ContentConfig{
		NegotiatedSerializer: scheme.Codecs.WithoutConversion(),
		GroupVersion:         &schema.GroupVersion{Version: "v1"},
	}
	c.config.APIPath = "/api"

	opts := &attach.AttachOptions{
		StreamOptions: exec.StreamOptions{
			// TODO: not sure how this has worked all this time for non-default namespaces?
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
		Attach:     &attach.DefaultRemoteAttach{},
		AttachFunc: attach.DefaultAttachFunc,
		// TODO: parameterize
		GetPodTimeout: time.Second * 10,
	}

	opts.Config = c.config
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

// Client requirements
//
// Check ns exists
// Check ws exists and is healthy
// Create command
// Create configmap
// Wait until pod is ready and running
// Get logs (kc)
// Attach (config)
// Set annotation on command resource
