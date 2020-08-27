package launcher

import (
	"context"

	"github.com/apex/log"
	"github.com/leg100/stok/api/command"
	"github.com/leg100/stok/api/v1alpha1"
	"github.com/leg100/stok/scheme"
	"github.com/leg100/stok/version"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (t *Launcher) createCommand(rc client.Client, name, configMapName string) (command.Interface, error) {
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
		// Name of the application
		"app":                          "stok",
		"app.kubernetes.io/name":       "stok",
		"app.kubernetes.io/part-of":    "stok",
		"app.kubernetes.io/managed-by": "stok",

		// Unique name of instance within application
		"app.kubernetes.io/instance": name,

		// Current version of application
		"version":                   version.Version,
		"app.kubernetes.io/version": version.Version,

		// Component within architecture
		"component":                   "command",
		"app.kubernetes.io/component": "command",

		// Workspace that this resource relates to
		"workspace":                    t.Workspace,
		"stok.goalspike.com/workspace": t.Workspace,

		// Comamnd that this resource relates to
		"command":                    name,
		"stok.goalspike.com/command": name,
	})
	cmd.SetWorkspace(t.Workspace)

	cmd.SetAnnotations(map[string]string{v1alpha1.WaitAnnotationKey: "true"})

	cmd.SetTimeoutQueue(t.TimeoutQueue.String())
	cmd.SetTimeoutClient(t.TimeoutClient.String())
	cmd.SetArgs(t.Args)
	cmd.SetDebug(t.Debug)
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

func (t *Launcher) createConfigMap(rc client.Client, tarball []byte, name, keyName string) (*corev1.ConfigMap, error) {
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: t.Namespace,
			Labels: map[string]string{
				// Name of the application
				"app":                          "stok",
				"app.kubernetes.io/name":       "stok",
				"app.kubernetes.io/part-of":    "stok",
				"app.kubernetes.io/managed-by": "stok",

				// Unique name of instance within application
				"app.kubernetes.io/instance": name,

				// Current version of application
				"version":                   version.Version,
				"app.kubernetes.io/version": version.Version,

				// Component within architecture
				"component":                   "archive",
				"app.kubernetes.io/component": "archive",

				// Workspace that this resource relates to
				"workspace":                    t.Workspace,
				"stok.goalspike.com/workspace": t.Workspace,

				// Comamnd that this resource relates to
				"command":                    name,
				"stok.goalspike.com/command": name,
			},
		},
		BinaryData: map[string][]byte{
			keyName: tarball,
		},
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
