package cmd

import (
	"bytes"
	"context"

	"github.com/apex/log"
	"github.com/leg100/stok/constants"
	"github.com/spf13/viper"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubectl/pkg/scheme"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (app *App) CreateCommand() error {
	app.Command.SetGenerateName("stok-")
	app.Command.SetNamespace(app.Namespace)
	app.Command.SetLabels(map[string]string{
		"app":       "stok",
		"workspace": app.Workspace,
	})
	app.Command.SetArgs(app.Args)
	app.Command.SetTimeoutClient(viper.GetString("timeout-client"))
	app.Command.SetTimeoutQueue(viper.GetString("timeout-queue"))

	if err := app.Client.Create(context.Background(), app.Command); err != nil {
		return err
	}

	log.WithFields(log.Fields{
		"type":      "command",
		"name":      app.Command.GetName(),
		"namespace": app.Namespace,
	}).Debug("resource created")

	return nil
}

func (app *App) CreateConfigMap(tarball *bytes.Buffer) (*corev1.ConfigMap, error) {
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      app.Command.GetName(),
			Namespace: app.Namespace,
			Labels: map[string]string{
				"app":       "stok",
				"workspace": app.Workspace,
				"command":   app.Command.GetName(),
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
	if err := controllerutil.SetControllerReference(app.Command, configMap, scheme.Scheme); err != nil {
		return nil, err
	}

	configMap, err := app.KubeClient.CoreV1().ConfigMaps(app.Namespace).Create(configMap)
	if err != nil {
		return nil, err
	}

	log.WithFields(log.Fields{
		"type":      "configmap",
		"name":      configMap.GetName(),
		"namespace": app.Namespace,
	}).Debug("resource created")

	return configMap, nil
}
