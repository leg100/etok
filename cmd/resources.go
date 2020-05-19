package cmd

import (
	"bytes"
	"context"

	"github.com/apex/log"
	"github.com/leg100/stok/constants"
	"github.com/leg100/stok/pkg/controller/command"
	"github.com/spf13/viper"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/kubectl/pkg/scheme"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (app *app) addToCleanup(resource runtime.Object) {
	app.resources = append(app.resources, resource)
}

func (app *app) cleanup() {
	for _, r := range app.resources {
		app.client.Delete(context.TODO(), r)
	}
}

func (app *app) createCommand(command command.Command) error {
	command.SetGenerateName("stok" + "-" + app.crd.Name + "-")
	command.SetNamespace(viper.GetString("namespace"))
	command.SetLabels(map[string]string{
		"app":       "stok",
		"workspace": viper.GetString("workspace"),
	})
	command.SetArgs(app.args)
	command.SetTimeoutClient(viper.GetString("timeout-client"))
	command.SetTimeoutQueue(viper.GetString("timeout-queue"))

	if err := app.client.Create(context.Background(), command); err != nil {
		return err
	}

	app.addToCleanup(command)

	log.WithFields(log.Fields{
		"type":      "command",
		"name":      command.GetName(),
		"namespace": viper.GetString("namespace"),
	}).Debug("resource created")

	return nil
}

func (app *app) createConfigMap(command command.Command, tarball *bytes.Buffer) (*corev1.ConfigMap, error) {
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      command.GetName(),
			Namespace: viper.GetString("namespace"),
			Labels: map[string]string{
				"app":       "stok",
				"workspace": viper.GetString("workspace"),
				"command":   command.GetName(),
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

	configMap, err := app.kubeClient.CoreV1().ConfigMaps(viper.GetString("namespace")).Create(configMap)
	if err != nil {
		return nil, err
	}

	log.WithFields(log.Fields{
		"type":      "configmap",
		"name":      configMap.GetName(),
		"namespace": viper.GetString("namespace"),
	}).Debug("resource created")

	return configMap, nil
}
