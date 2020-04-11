package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/leg100/stok/pkg/apis"
	terraformv1alpha1 "github.com/leg100/stok/pkg/apis/terraform/v1alpha1"
	"github.com/leg100/stok/util"
	"github.com/leg100/stok/util/slice"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubectl/pkg/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	runtimeconfig "sigs.k8s.io/controller-runtime/pkg/client/config"
)

var TERRAFORM_COMMANDS_THAT_USE_STATE = []string{
	"init",
	"apply",
	"destroy",
	"env",
	"import",
	"graph",
	"output",
	"plan",
	"push",
	"refresh",
	"show",
	"taint",
	"untaint",
	"validate",
	"force-unlock",
	"state",
}

const (
	exitCodeErr       = 1
	exitCodeInterrupt = 2
)

func main() {
	if err := parseArgs(os.Args[1:]); err != nil {
		handleError(err)
	}
}

func parseArgs(args []string) error {
	if len(args) > 0 && slice.ContainsString(TERRAFORM_COMMANDS_THAT_USE_STATE, args[0]) {
		return runRemote(args)
	}

	return runLocal(args)
}

func handleError(err error) {
	if exiterr, ok := err.(*exec.ExitError); ok {
		// terraform exited with non-zero exit code
		os.Exit(exiterr.ExitCode())
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(exitCodeErr)
	}
}

func runLocal(args []string) error {
	cmd := exec.Command("terraform", args...)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

type App struct {
	Resources []runtime.Object
	Client    client.Client
}

func (app *App) AddToCleanup(resource runtime.Object) {
	app.Resources = append(app.Resources, resource)
}

func (app *App) Cleanup() {
	for _, r := range app.Resources {
		app.Client.Delete(context.TODO(), r)
	}
}

func runRemote(args []string) error {
	app := App{}
	defer app.Cleanup()

	// adds core GVKs
	s := scheme.Scheme
	// adds CRD GVKs
	apis.AddToScheme(s)

	config := runtimeconfig.GetConfigOrDie()
	cl, err := client.New(config, client.Options{Scheme: s})
	if err != nil {
		return err
	}
	app.Client = cl

	stopChan := make(chan os.Signal, 1)
	// Stop on SIGINTs and SIGTERMs.
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-stopChan
		app.Cleanup()
		os.Exit(exitCodeInterrupt)
	}()

	// create tar
	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	filenames, err := filepath.Glob("*.tf")
	if err != nil {
		return err
	}
	tar, err := util.Create(wd, filenames)
	if err != nil {
		return err
	}

	// get k8s namespace or set default
	namespace := os.Getenv("STOK_NAMESPACE")
	if namespace == "" {
		namespace = "default"
	}

	// get workspace or set default
	workspace := os.Getenv("STOK_WORKSPACE")
	if workspace == "" {
		workspace = "default"
	}

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "stok-",
			Namespace:    namespace,
			Labels: map[string]string{
				"app": "stok",
			},
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		BinaryData: map[string][]byte{
			"tarball.tar.gz": tar.Bytes(),
		},
	}

	err = cl.Create(context.Background(), configMap)
	if err != nil {
		return err
	}
	app.AddToCleanup(configMap)

	command := &terraformv1alpha1.Command{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMap.GetName(),
			Namespace: namespace,
			Labels: map[string]string{
				"workspace": workspace,
			},
		},
		Spec: terraformv1alpha1.CommandSpec{
			Command:      []string{"terraform"},
			Args:         args,
			ConfigMap:    configMap.GetName(),
			ConfigMapKey: "tarball.tar.gz",
		},
	}

	err = cl.Create(context.Background(), command, &client.CreateOptions{})
	if err != nil {
		return err
	}
	app.AddToCleanup(command)

	clientset, err := kubernetes.NewForConfig(config)
	name, err := waitForPod(clientset, command.GetName(), namespace)
	if err != nil {
		return err
	}
	req := clientset.CoreV1().Pods(namespace).GetLogs(name, &v1.PodLogOptions{Follow: true})
	logs, err := req.Stream()
	if err != nil {
		return err
	}
	defer logs.Close()

	_, err = io.Copy(os.Stdout, logs)
	if err != nil {
		return err
	}
	return nil
}

func waitForPod(client *kubernetes.Clientset, podName string, namespace string) (string, error) {
	// TODO: add timeout
	fieldSelector := fmt.Sprintf("metadata.name=%v", podName)
	watcher, err := client.CoreV1().Pods(namespace).Watch(metav1.ListOptions{FieldSelector: fieldSelector})
	if err != nil {
		return "", errors.Wrap(err, "cannot create pod event watcher")
	}
	eventChan := watcher.ResultChan()

	for e := range eventChan {
		if e.Object == nil {
			return "", errors.New("Empty pod received")
		}

		pod, ok := e.Object.(*v1.Pod)
		if !ok {
			return "", errors.New("Received obj that is not a pod")
		}

		//fmt.Printf("%s - %s\n", e.Type, pod.Name)

		switch e.Type {
		case watch.Added:
			continue
		case watch.Modified:
			switch pod.Status.Phase {
			case v1.PodRunning:
				return pod.Name, nil
			case v1.PodSucceeded:
				return pod.Name, nil
			case v1.PodPending:
				continue
			default:
				return "", fmt.Errorf("unexpected pod status: %+v", pod.Status.Phase)
			}
		default:
			return "", fmt.Errorf("Unexpected event %+v on pod %+v", e.Type, pod.Name)
		}
	}
	return "", errors.New("unexpected result")
}
