package queue

import (
	"sigs.k8s.io/controller-runtime/pkg/client"
)

//go:generate go run generate.go

var AddToQueueFuncs []func(client.Client, string, string) ([]string, error)

func AddToQueue(cl client.Client, workspace, namespace string) ([]string, error) {
	queue := []string{}
	for _, f := range AddToQueueFuncs {
		cmds, err := f(cl, workspace, namespace)
		if err != nil {
			return nil, err
		}
		queue = append(queue, cmds...)
	}
	return queue, nil
}
