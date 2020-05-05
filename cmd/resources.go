package cmd

import (
	"bytes"
	"context"

	"github.com/apex/log"
	"github.com/leg100/stok/pkg/apis/stok/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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

	log.WithFields(log.Fields{
		"type":      "configmap",
		"name":      configMap.GetName(),
		"namespace": app.Namespace,
	}).Debug("resource created")

	return configMap.GetName(), nil
}

func (app *App) CreateCommand(name string) (*v1alpha1.Command, error) {
	command := &v1alpha1.Command{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: app.Namespace,
			Labels: map[string]string{
				"workspace": app.Workspace,
			},
		},
		Spec: v1alpha1.CommandSpec{
			Command:   app.Command,
			Args:      app.Args,
			ConfigMap: name,
			// TODO: use constant
			ConfigMapKey: "tarball.tar.gz",
		},
	}

	if err := app.Client.Create(context.Background(), command); err != nil {
		return command, err
	}
	app.AddToCleanup(command)

	log.WithFields(log.Fields{
		"type":      "command",
		"name":      command.GetName(),
		"namespace": app.Namespace,
	}).Debug("resource created")

	return command, nil
}
