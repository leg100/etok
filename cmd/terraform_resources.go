package cmd

import (
	"bytes"
	"fmt"

	"github.com/apex/log"
	"github.com/iancoleman/strcase"
	"github.com/leg100/stok/constants"
	"github.com/leg100/stok/crdinfo"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubectl/pkg/scheme"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (t *terraformCmd) createCommand(client dynamic.Interface, crd crdinfo.CRDInfo) (*unstructured.Unstructured, error) {
	commandRes := schema.GroupVersionResource{Group: "stok.goalspike.com", Version: "v1alpha1", Resource: crd.APIPlural}
	command := &unstructured.Unstructured{}
	command.SetNamespace(t.Workspace.getNamespace())
	command.SetAPIVersion("stok.goalspike.com/v1alpha1")
	command.SetKind(strcase.ToCamel(t.Command))
	command.SetGenerateName(fmt.Sprintf("stok-%s-", t.Command))
	command.SetLabels(map[string]string{
		"app":       "stok",
		"workspace": t.Workspace.getWorkspace(),
	})
	command.Object["spec"] = map[string]interface{}{
		"timeoutqueue":  t.TimeoutQueue.String(),
		"timeoutclient": t.TimeoutClient.String(),
	}
	unstructured.SetNestedStringSlice(command.Object, t.Args, "spec", "args")

	command, err := client.Resource(commandRes).Namespace(t.Workspace.getNamespace()).Create(command, metav1.CreateOptions{})
	if err != nil {
		return command, err
	}

	log.WithFields(log.Fields{
		"type":      "command",
		"name":      command.GetName(),
		"namespace": t.Workspace.getNamespace(),
	}).Debug("resource created")

	return command, nil
}

func (t *terraformCmd) createConfigMap(client kubernetes.Interface, command metav1.Object, tarball *bytes.Buffer) (*corev1.ConfigMap, error) {
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      command.GetName(),
			Namespace: t.Workspace.getNamespace(),
			Labels: map[string]string{
				"app":       "stok",
				"workspace": t.Workspace.getWorkspace(),
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

	configMap, err := client.CoreV1().ConfigMaps(t.Workspace.getNamespace()).Create(configMap)
	if err != nil {
		return nil, err
	}

	log.WithFields(log.Fields{
		"type":      "configmap",
		"name":      configMap.GetName(),
		"namespace": t.Workspace.getNamespace(),
	}).Debug("resource created")

	return configMap, nil
}
