package cmd

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/apex/log"
	"github.com/leg100/stok/constants"
	"github.com/leg100/stok/pkg/apis/stok/v1alpha1"
	"github.com/leg100/stok/pkg/apis/stok/v1alpha1/command"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubectl/pkg/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (t *terraformCmd) createCommand(rc client.Client) (command.Interface, error) {
	obj, err := t.scheme.New(v1alpha1.SchemeGroupVersion.WithKind(t.Kind))
	if err != nil {
		return nil, err
	}

	cmd := obj.(command.Interface)

	// TODO: Why is this necessary after having used scheme.New? If so, leave comment explaining why
	cmd.SetGroupVersionKind(v1alpha1.SchemeGroupVersion.WithKind(t.Kind))
	cmd.SetNamespace(t.Namespace)

	// TODO: Generate our own unique suffix. Currently we rely on k8s to generate one, and then
	// reuse it for the configmap, which means the configmap has to be created *afterwards*. If we
	// generated our own suffix, then we could create the configmap in parallel, speeding up
	// deployment (the configmap could contain up to 1Mb of data to upload).
	cmd.SetGenerateName(fmt.Sprintf("stok-%s-", strings.ToLower(t.Kind)))
	cmd.SetLabels(map[string]string{
		"app":       "stok",
		"workspace": t.Workspace,
	})

	cmd.SetAnnotations(map[string]string{v1alpha1.CommandWaitAnnotationKey: "true"})

	cmd.SetTimeoutQueue(t.TimeoutQueue.String())
	cmd.SetTimeoutClient(t.TimeoutClient.String())
	cmd.SetArgs(t.Args)
	cmd.SetDebug(t.debug)

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

func (t *terraformCmd) createConfigMap(rc client.Client, command metav1.Object, tarball *bytes.Buffer) (*corev1.ConfigMap, error) {
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

	if err := rc.Create(context.TODO(), configMap); err != nil {
		return nil, err
	}

	log.WithFields(log.Fields{
		"namespace": t.Namespace,
		"configmap": configMap.GetName(),
	}).Debug("resource created")

	return configMap, nil
}
