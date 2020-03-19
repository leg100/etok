package main

import (
	"context"
	"fmt"
	"io"
	"os"

	crdapi "github.com/leg100/terraform-operator/pkg/apis"
	terraformv1alpha1 "github.com/leg100/terraform-operator/pkg/apis/terraform/v1alpha1"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
	runtimeconfig "sigs.k8s.io/controller-runtime/pkg/client/config"
)

func newClient() error {
	namespace := "default"

	// create Command CRD (and defer deletion)
	scheme := runtime.NewScheme()
	crdapi.AddToScheme(scheme)

	var cl client.Client
	config := runtimeconfig.GetConfigOrDie()
	cl, err := client.New(config, client.Options{Scheme: scheme})
	if err != nil {
		return err
	}

	command := &terraformv1alpha1.Command{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "command-",
			Namespace:    namespace,
		},
		Spec: terraformv1alpha1.CommandSpec{
			Workspace: "example-workspace",
			Command:   []string{"sh"},
			Args:      []string{"-c", "for i in $(seq 1 3); do echo $i; sleep 1; done"},
		},
	}

	err = cl.Create(context.Background(), command, &client.CreateOptions{})
	if err != nil {
		return err
	}
	defer cl.Delete(context.Background(), command, &client.DeleteOptions{})

	clientset, err := kubernetes.NewForConfig(config)
	name, err := waitForPod(clientset, command.GetName())
	if err != nil {
		return err
	}
	req := clientset.CoreV1().Pods("default").GetLogs(name, &v1.PodLogOptions{Follow: true})
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

func waitForPod(client *kubernetes.Clientset, podName string) (string, error) {
	// TODO: add timeout
	fieldSelector := fmt.Sprintf("metadata.name=%v", podName)
	watcher, err := client.CoreV1().Pods("default").Watch(metav1.ListOptions{FieldSelector: fieldSelector})
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

		fmt.Printf("%s - %s\n", e.Type, pod.Name)

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
