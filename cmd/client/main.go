package main

import (
	"bufio"
	"context"
	"fmt"

	crdapi "github.com/leg100/terraform-operator/pkg/apis"
	terraformv1alpha1 "github.com/leg100/terraform-operator/pkg/apis/terraform/v1alpha1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	watch "k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

func main() {
	namespace := "default"

	// create Command CRD (and defer deletion)
	scheme := runtime.NewScheme()
	clientgoscheme.AddToScheme(scheme)
	crdapi.AddToScheme(scheme)

	cl, err := client.New(config.GetConfigOrDie(), client.Options{Scheme: scheme})
	if err != nil {
		panic(err.Error())
	}

	command := &terraformv1alpha1.Command{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-command",
			Namespace: namespace,
		},
		Spec: terraformv1alpha1.CommandSpec{
			Workspace: "example-workspace",
			Command:   []string{"sh"},
			Args:      []string{"-c", "for i in $(seq 1 3); do echo $i; sleep 1; done"},
		},
	}

	err = cl.Create(context.Background(), command, &client.CreateOptions{})
	if err != nil {
		panic(err.Error())
	}
	defer cl.Delete(context.Background(), command, &client.DeleteOptions{})

	// create k8s client
	client, err := NewClient()
	if err != nil {
		panic(err.Error())
	}

	watcher, err := client.goClient.CoreV1().Pods(namespace).Watch(metav1.ListOptions{LabelSelector: "app=my-command"})
	eventChan := watcher.ResultChan()

	for e := range eventChan {
		if e.Object == nil {
			return
		}

		pod, ok := e.Object.(*v1.Pod)
		if !ok {
			fmt.Println("not a pod")
			return
		}

		fmt.Printf("%s - %s\n", e.Type, pod.Name)
		switch e.Type {
		case watch.Added:
			continue
		case watch.Modified:
			switch pod.Status.Phase {
			case v1.PodRunning:
				fmt.Printf("%s - %s\n", pod.Name, pod.Status.Phase)
				logs, err := client.goClient.CoreV1().Pods(namespace).GetLogs(pod.Name, &v1.PodLogOptions{
					Container: "terraform",
					Follow:    true,
				}).Stream()
				if err != nil {
					panic(err.Error())
				}
				defer logs.Close()

				sc := bufio.NewScanner(logs)
				for sc.Scan() {
					fmt.Println(sc.Text())
				}
				return
			}
		default:
			return
		}
	}
}

func NewClient() (*Client, error) {
	clientset, err := kubernetes.NewForConfig(config.GetConfigOrDie())
	if err != nil {
		return nil, err
	}

	return &Client{
		goClient: clientset,
	}, nil
}

type Client struct {
	goClient *kubernetes.Clientset
}
