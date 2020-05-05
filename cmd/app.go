package cmd

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/apex/log"
	"github.com/leg100/stok/pkg/apis"
	"github.com/leg100/stok/pkg/apis/stok/v1alpha1"
	v1alpha1clientset "github.com/leg100/stok/pkg/client/clientset/typed/stok/v1alpha1"
	"github.com/leg100/stok/util"
	"github.com/operator-framework/operator-sdk/pkg/status"
	"golang.org/x/crypto/ssh/terminal"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	watchtools "k8s.io/client-go/tools/watch"
	"k8s.io/client-go/util/retry"
	"k8s.io/kubectl/pkg/cmd/attach"
	"k8s.io/kubectl/pkg/cmd/exec"
	"k8s.io/kubectl/pkg/scheme"
	"k8s.io/kubectl/pkg/util/interrupt"
	"sigs.k8s.io/controller-runtime/pkg/client"
	runtimeconfig "sigs.k8s.io/controller-runtime/pkg/client/config"
)

func InitClient() (*client.Client, *kubernetes.Clientset, *v1alpha1clientset.StokV1alpha1Client, error) {
	// adds core GVKs
	s := scheme.Scheme
	// adds CRD GVKs
	apis.AddToScheme(s)

	// controller-runtime dynamic client
	config := runtimeconfig.GetConfigOrDie()
	client, err := client.New(config, client.Options{Scheme: s})
	if err != nil {
		return nil, nil, nil, err
	}

	// client-go client
	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, nil, nil, err
	}

	// generated clientset
	clientset, err := v1alpha1clientset.NewForConfig(config)
	if err != nil {
		return nil, nil, nil, err
	}

	return &client, kubeClient, clientset, nil
}

type App struct {
	Workspace string
	Namespace string
	Tarball   *bytes.Buffer
	Command   []string
	Args      []string
	Resources []runtime.Object
	// embed?
	Client client.Client
	// embed?
	KubeClient     kubernetes.Interface
	Clientset      v1alpha1clientset.StokV1alpha1Interface
	PodWaitTimeout time.Duration
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

	command, err := app.CreateCommand()
	if err != nil {
		return err
	}
	// the pod name is the same as the command name
	podName := command.Name

	_, err = app.CreateConfigMap(command, tarball)
	if err != nil {
		return err
	}

	log.WithFields(log.Fields{
		"id": command.GetName(),
	}).Info("Initialising")

	log.Debug("Monitoring workspace queue...")
	_, err = app.WaitForWorkspaceReady(command.Name, time.Duration(queueTimeout)*time.Second)
	if err != nil {
		if err == wait.ErrWaitTimeout {
			return fmt.Errorf("timed out waiting for workspace to be available")
		} else {
			return err
		}
	}

	log.Debug("Waiting for pod to be running and ready...")
	pod, err := app.WaitForPod(podName, podRunningAndReady, app.PodWaitTimeout)
	if err != nil {
		if err == wait.ErrWaitTimeout {
			return fmt.Errorf("timed out waiting for pod %s to be running and ready", podName)
		} else {
			return err
		}
	}

	log.Debug("Retrieving log stream...")
	req := app.KubeClient.CoreV1().Pods(app.Namespace).GetLogs(podName, &corev1.PodLogOptions{Follow: true})
	logs, err := req.Stream()
	if err != nil {
		return err
	}
	defer logs.Close()

	done := make(chan error)
	go func() {
		log.WithFields(log.Fields{
			"pod": pod.GetName(),
		}).Info("Attaching")

		drawDivider()

		err := app.handleAttachPod(pod)
		if err != nil {
			log.Warn("Failed to attach to pod TTY; falling back to streaming logs")
			_, err = io.Copy(os.Stdout, logs)
			done <- err
		} else {
			done <- nil
		}
	}()

	// let operator know we're now streaming logs
	log.Debug("Telling the operator I'm ready to receive logs...")
	retry.RetryOnConflict(retry.DefaultRetry, func() error {
		log.Debug("Attempt to update command resource...")
		key := types.NamespacedName{Name: command.Name, Namespace: app.Namespace}
		if err = app.Client.Get(context.Background(), key, command); err != nil {
			done <- err
		} else {
			annotations := command.GetAnnotations()
			if annotations == nil {
				annotations = make(map[string]string)
			}
			annotations["stok.goalspike.com/client"] = "Ready"
			command.SetAnnotations(annotations)

			return app.Client.Update(context.Background(), command)
		}
		return nil
	})

	return <-done
}

// draw divider the width of the terminal
func drawDivider() {
	width := 80

	if terminal.IsTerminal(int(os.Stdin.Fd())) {
		width, _, _ = terminal.GetSize(0)
	}
	fmt.Println(strings.Repeat("-", width))
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

// waitForWorkspaceReady watches the command until the WorkspaceReady condition is true
func (app *App) WaitForWorkspaceReady(name string, timeout time.Duration) (*v1alpha1.Command, error) {
	ctx, cancel := watchtools.ContextWithOptionalTimeout(context.Background(), timeout)
	defer cancel()

	fieldSelector := fields.OneTermEqualSelector("metadata.name", name).String()
	lw := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			options.FieldSelector = fieldSelector
			return app.Clientset.Commands(app.Namespace).List(options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			options.FieldSelector = fieldSelector
			return app.Clientset.Commands(app.Namespace).Watch(options)
		},
	}

	intr := interrupt.New(nil, cancel)
	var result *v1alpha1.Command
	err := intr.Run(func() error {
		ev, err := watchtools.UntilWithSync(ctx, lw, &v1alpha1.Command{}, nil, func(ev watch.Event) (bool, error) {
			switch ev.Type {
			case watch.Deleted:
				return false, errors.NewNotFound(schema.GroupResource{Resource: "commands"}, "")
			}
			switch t := ev.Object.(type) {
			case *v1alpha1.Command:
				condition := t.Status.Conditions.GetCondition(status.ConditionType("WorkspaceReady"))
				if condition != nil {
					status, err := strconv.ParseBool(string(condition.Status))
					if err != nil {
						return false, fmt.Errorf("Could not parse WorkspaceReady condition status")
					}

					log.Debugf("reason: %s", string(condition.Reason))
					switch string(condition.Reason) {
					case "WorkspaceNotFound":
						return false, fmt.Errorf(condition.Message)
					case "SecretNotFound":
						return false, fmt.Errorf(condition.Message)
					case "Queued":
						log.WithFields(log.Fields{
							"msg": condition.Message,
						}).Info("Status update")
						return status, nil
					default:
						log.WithFields(log.Fields{
							"msg": condition.Message,
						}).Debug("Status update")
						return status, nil
					}
				}
			}

			return false, nil
		})
		if ev != nil {
			result = ev.Object.(*v1alpha1.Command)
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
func (app *App) Write(in []byte) (int, error) {
	s := strings.TrimSpace(string(in))
	log.Warn(s)
	return 0, nil
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
		case corev1.PodSucceeded:
			return false, ErrPodCompleted
		case corev1.PodFailed:
			return false, fmt.Errorf(t.Status.ContainerStatuses[0].State.Terminated.Message)
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
