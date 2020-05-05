package cmd

import (
	"bytes"
	"context"

	"github.com/apex/log"
	"github.com/leg100/stok/constants"
	"github.com/leg100/stok/pkg/apis/stok/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubectl/pkg/scheme"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (app *App) CreateCommand() (*v1alpha1.Command, error) {
	command := &v1alpha1.Command{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "stok-",
			Namespace:    app.Namespace,
			Labels: map[string]string{
				"app":       "stok",
				"workspace": app.Workspace,
			},
		},
		Spec: v1alpha1.CommandSpec{
			Command: app.Command,
			Args:    app.Args,
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

func (app *App) CreateConfigMap(command *v1alpha1.Command, tarball *bytes.Buffer) (*corev1.ConfigMap, error) {
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      command.Name,
			Namespace: app.Namespace,
			Labels: map[string]string{
				"app":       "stok",
				"workspace": app.Workspace,
				"command":   command.Name,
			},
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		BinaryData: map[string][]byte{
			constants.Tarball: tarball.Bytes(),
		},
	}

	// Set Command instance as the owner and controller
	if err := controllerutil.SetControllerReference(command, configMap, scheme.Scheme); err != nil {
		return nil, err
	}

	configMap, err := app.KubeClient.CoreV1().ConfigMaps(app.Namespace).Create(configMap)
	if err != nil {
		return nil, err
	}
	app.AddToCleanup(configMap)

	log.WithFields(log.Fields{
		"type":      "configmap",
		"name":      configMap.GetName(),
		"namespace": app.Namespace,
	}).Debug("resource created")

	return configMap, nil
}
