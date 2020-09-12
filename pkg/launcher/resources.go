package launcher

import (
	"context"

	"github.com/apex/log"
	"github.com/leg100/stok/api/v1alpha1"
	"github.com/leg100/stok/version"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (t *Launcher) createRun(rc client.Client, name, configMapName string) (*v1alpha1.Run, error) {
	run := &v1alpha1.Run{}
	run.SetNamespace(t.Namespace)
	run.SetName(name)
	run.SetLabels(map[string]string{
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
	run.SetWorkspace(t.Workspace)

	run.SetAnnotations(map[string]string{v1alpha1.WaitAnnotationKey: "true"})

	run.SetTimeoutQueue(t.TimeoutQueue.String())
	run.SetTimeoutClient(t.TimeoutClient.String())
	run.SetArgs(t.Args)
	run.SetDebug(t.Debug)
	run.SetConfigMap(configMapName)
	run.SetConfigMapKey(v1alpha1.RunDefaultConfigMapKey)

	if err := rc.Create(context.TODO(), run); err != nil {
		return nil, err
	}

	log.WithFields(log.Fields{
		"namespace": t.Namespace,
		"name":      run.GetName(),
	}).Debug("resource created")

	return run, nil
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
