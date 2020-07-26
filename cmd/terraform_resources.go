package cmd

import (
	"context"

	"github.com/apex/log"
	"github.com/leg100/stok/api/command"
	"github.com/leg100/stok/api/v1alpha1"
	"github.com/leg100/stok/scheme"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (t *terraformCmd) createCommand(rc client.Client, name, configMapName string) (command.Interface, error) {
	obj, err := scheme.Scheme.New(v1alpha1.SchemeGroupVersion.WithKind(t.Kind))
	if err != nil {
		return nil, err
	}

	cmd := obj.(command.Interface)

	// TODO: Why is this necessary after having used scheme.New? If so, leave comment explaining why
	cmd.SetGroupVersionKind(v1alpha1.SchemeGroupVersion.WithKind(t.Kind))
	cmd.SetNamespace(t.Namespace)
	cmd.SetName(name)
	cmd.SetLabels(map[string]string{
		"app":       "stok",
		"workspace": t.Workspace,
	})

	cmd.SetAnnotations(map[string]string{v1alpha1.CommandWaitAnnotationKey: "true"})

	cmd.SetTimeoutQueue(t.TimeoutQueue.String())
	cmd.SetTimeoutClient(t.TimeoutClient.String())
	cmd.SetArgs(t.Args)
	cmd.SetDebug(t.debug)
	cmd.SetConfigMap(configMapName)
	cmd.SetConfigMapKey(v1alpha1.CommandDefaultConfigMapKey)

	err = rc.Create(context.TODO(), cmd)
	if err != nil {
		return nil, err
	}

	log.WithFields(log.Fields{
		"namespace": t.Namespace,
		"name":      cmd.GetName(),
		"kind":      t.Kind,
	}).Debug("resource created")

	return cmd, nil
}

func (t *terraformCmd) createConfigMap(rc client.Client, command metav1.Object, tarball []byte, name, keyName string) (*corev1.ConfigMap, error) {
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: t.Namespace,
			Labels: map[string]string{
				"app":       "stok",
				"workspace": t.Workspace,
				"command":   command.GetName(),
			},
		},
		BinaryData: map[string][]byte{
			keyName: tarball,
		},
	}

	// Set Command instance as the owner and controller
	if err := controllerutil.SetControllerReference(command, configMap, scheme.Scheme); err != nil {
		return nil, err
	}

	if err := rc.Create(context.TODO(), configMap); err != nil {
		return nil, err
	}

	log.WithFields(log.Fields{
		"namespace": t.Namespace,
		"configmap": configMap.GetName(),
	}).Debug("resource created")

	return configMap, nil
}
