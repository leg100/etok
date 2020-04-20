package app

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/leg100/stok/pkg/apis"
	"github.com/leg100/stok/pkg/apis/terraform/v1alpha1"
	"github.com/leg100/stok/util"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	watchtools "k8s.io/client-go/tools/watch"
	"k8s.io/kubectl/pkg/cmd/attach"
	"k8s.io/kubectl/pkg/cmd/exec"
	"k8s.io/kubectl/pkg/scheme"
	"k8s.io/kubectl/pkg/util/interrupt"
	"sigs.k8s.io/controller-runtime/pkg/client"
	runtimeconfig "sigs.k8s.io/controller-runtime/pkg/client/config"
)

func InitClient() (*client.Client, *kubernetes.Clientset, error) {
	// adds core GVKs
	s := scheme.Scheme
	// adds CRD GVKs
	apis.AddToScheme(s)

	config := runtimeconfig.GetConfigOrDie()
	client, err := client.New(config, client.Options{Scheme: s})
	if err != nil {
		return nil, nil, err
	}

	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, nil, err
	}

	return &client, kubeClient, nil
}

type App struct {
	Workspace string
	Namespace string
	Tarball   *bytes.Buffer
	Args      []string
	Resources []runtime.Object
	// embed?
	Client client.Client
	// embed?
	KubeClient kubernetes.Interface
}

func (app *App) AddToCleanup(resource runtime.Object) {
	app.Resources = append(app.Resources, resource)
}

func (app *App) Cleanup() {
	for _, r := range app.Resources {
		app.Client.Delete(context.TODO(), r)
	}
}

func (app *App) Run() error {
	defer app.Cleanup()

	stopChan := make(chan os.Signal, 1)
	// Stop on SIGINTs and SIGTERMs.
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-stopChan
		app.Cleanup()
		os.Exit(3)
	}()

	tarball, err := CreateTar()
	if err != nil {
		return err
	}

	name, err := app.CreateConfigMap(tarball)
	if err != nil {
		return err
	}

	err = app.CreateCommand(name)
	if err != nil {
		return err
	}

	// TODO: make timeout configurable
	pod, err := app.WaitForPod(name, podRunningAndReady, 10*time.Second)
	if err != nil {
		return err
	}

	app.handleAttachPod(pod)
	if err != nil {
		return err
	}

	return nil
}

func (app *App) CheckWorkspaceExists() (bool, error) {
	key := types.NamespacedName{Namespace: app.Namespace, Name: app.Workspace}
	err := app.Client.Get(context.Background(), key, &v1alpha1.Workspace{})
	if err != nil {
		if errors.IsNotFound(err) {
			return false, nil
		} else {
			// something unexpected happened
			return false, err
		}
	}
	return true, nil
}

// TODO: unit test
func CreateTar() (*bytes.Buffer, error) {
	// create tar
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	filenames, err := filepath.Glob("*.tf")
	if err != nil {
		return nil, err
	}
	tar, err := util.Create(wd, filenames)
	if err != nil {
		return nil, err
	}
	return tar, nil
}

func (app *App) CreateConfigMap(tarball *bytes.Buffer) (string, error) {
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "stok-",
			Namespace:    app.Namespace,
			Labels: map[string]string{
				"app": "stok",
			},
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		BinaryData: map[string][]byte{
			// TODO: use constant
			"tarball.tar.gz": tarball.Bytes(),
		},
	}

	configMap, err := app.KubeClient.CoreV1().ConfigMaps(app.Namespace).Create(configMap)
	if err != nil {
		return "", err
	}
	app.AddToCleanup(configMap)

	return configMap.GetName(), nil
}

func (app *App) CreateCommand(name string) error {
	command := &v1alpha1.Command{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: app.Namespace,
			Labels: map[string]string{
				"workspace": app.Workspace,
			},
		},
		Spec: v1alpha1.CommandSpec{
			Command:   []string{"terraform"},
			Args:      app.Args,
			ConfigMap: name,
			// TODO: use constant
			ConfigMapKey: "tarball.tar.gz",
		},
	}

	if err := app.Client.Create(context.Background(), command); err != nil {
		return err
	}
	app.AddToCleanup(command)

	return nil
}

// waitForPod watches the given pod until the exitCondition is true
func (app *App) WaitForPod(name string, exitCondition watchtools.ConditionFunc, timeout time.Duration) (*corev1.Pod, error) {
	ctx, cancel := watchtools.ContextWithOptionalTimeout(context.Background(), timeout)
	defer cancel()

	fieldSelector := fields.OneTermEqualSelector("metadata.name", name).String()
	lw := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			options.FieldSelector = fieldSelector
			return app.KubeClient.CoreV1().Pods(app.Namespace).List(options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			options.FieldSelector = fieldSelector
			return app.KubeClient.CoreV1().Pods(app.Namespace).Watch(options)
		},
	}

	intr := interrupt.New(nil, cancel)
	var result *corev1.Pod
	err := intr.Run(func() error {
		ev, err := watchtools.UntilWithSync(ctx, lw, &corev1.Pod{}, nil, func(ev watch.Event) (bool, error) {
			return exitCondition(ev)
		})
		if ev != nil {
			result = ev.Object.(*corev1.Pod)
		}
		return err
	})

	return result, err
}

func (app *App) handleAttachPod(pod *corev1.Pod) error {
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
				ErrOut: os.Stderr,
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

// ErrPodCompleted is returned by PodRunning or PodContainerRunning to indicate that
// the pod has already reached completed state.
var ErrPodCompleted = fmt.Errorf("pod ran to completion")

// podRunningAndReady returns true if the pod is running and ready, false if the pod has not
// yet reached those states, returns ErrPodCompleted if the pod has run to completion, or
// an error in any other case.
func podRunningAndReady(event watch.Event) (bool, error) {
	switch event.Type {
	case watch.Deleted:
		return false, errors.NewNotFound(schema.GroupResource{Resource: "pods"}, "")
	}
	switch t := event.Object.(type) {
	case *corev1.Pod:
		switch t.Status.Phase {
		case corev1.PodFailed, corev1.PodSucceeded:
			return false, ErrPodCompleted
		case corev1.PodRunning:
			conditions := t.Status.Conditions
			if conditions == nil {
				return false, nil
			}
			for i := range conditions {
				if conditions[i].Type == corev1.PodReady &&
					conditions[i].Status == corev1.ConditionTrue {
					return true, nil
				}
			}
		}
	}
	return false, nil
}
