package cmd

import (
	"bytes"
	"context"
	"fmt"

	"github.com/apex/log"
	"github.com/leg100/stok/constants"
	"github.com/leg100/stok/crdinfo"
	"github.com/leg100/stok/pkg/apis/stok/v1alpha1/command"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubectl/pkg/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (t *terraformCmd) createCommand(rc client.Client, crd crdinfo.CRDInfo) (command.Interface, error) {
	obj, err := t.scheme.New(crd.GroupVersionKind())
	if err != nil {
		return nil, err
	}

	cmd := obj.(command.Interface)

	cmd.SetGroupVersionKind(crd.GroupVersionKind())
	cmd.SetNamespace(t.Namespace)
	cmd.SetGenerateName(fmt.Sprintf("stok-%s-", t.Command))
	cmd.SetLabels(map[string]string{
		"app":       "stok",
		"workspace": t.Workspace,
	})
	cmd.SetTimeoutQueue(t.TimeoutQueue.String())
	cmd.SetTimeoutClient(t.TimeoutClient.String())
	cmd.SetArgs(t.Args)

	err = rc.Create(context.TODO(), cmd)
	if err != nil {
		return nil, err
	}

	log.WithFields(log.Fields{
		t.Command:   cmd.GetName(),
		"namespace": t.Namespace,
	}).Debug("resource created")

	return cmd, nil
}

func (t *terraformCmd) createConfigMap(client kubernetes.Interface, command metav1.Object, tarball *bytes.Buffer) (*corev1.ConfigMap, error) {
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      command.GetName(),
			Namespace: t.Namespace,
			Labels: map[string]string{
				"app":       "stok",
				"workspace": t.Workspace,
				"command":   command.GetName(),
			},
		},
		BinaryData: map[string][]byte{
			constants.Tarball: tarball.Bytes(),
		},
	}

	// Set Command instance as the owner and controller
	if err := controllerutil.SetControllerReference(command, configMap, scheme.Scheme); err != nil {
		return nil, err
	}

	configMap, err := client.CoreV1().ConfigMaps(t.Namespace).Create(configMap)
	if err != nil {
		return nil, err
	}

	log.WithFields(log.Fields{
		"configmap": configMap.GetName(),
		"namespace": t.Namespace,
	}).Debug("resource created")

	return configMap, nil
}
