package launcher

import (
	"context"

	"github.com/leg100/stok/api/stok.goalspike.com/v1alpha1"
	"github.com/leg100/stok/pkg/log"
	"github.com/leg100/stok/version"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (o *LauncherOptions) createRun(ctx context.Context, name, configMapName string, isTTY bool) (*v1alpha1.Run, error) {
	run := &v1alpha1.Run{}
	run.SetNamespace(o.Namespace)
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
		"workspace":                    o.Workspace,
		"stok.goalspike.com/workspace": o.Workspace,

		// Comamnd that this resource relates to
		"command":                    name,
		"stok.goalspike.com/command": name,
	})
	run.SetWorkspace(o.Workspace)

	run.SetTimeoutClient(o.TimeoutClient.String())
	run.SetCommand(o.Command)
	run.SetArgs(o.args)
	run.SetDebug(o.Debug)
	run.SetConfigMap(configMapName)
	run.SetConfigMapKey(v1alpha1.RunDefaultConfigMapKey)

	if isTTY {
		run.AttachSpec.RequireMagicString = true
	}

	run, err := o.RunsClient(o.Namespace).Create(ctx, run, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	log.Debugf("created run %s/%s\n", o.Namespace, o.RunName)

	return run, nil
}

func (o *LauncherOptions) createConfigMap(ctx context.Context, tarball []byte, name, keyName string) error {
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: o.Namespace,
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
				"workspace":                    o.Workspace,
				"stok.goalspike.com/workspace": o.Workspace,

				// Comamnd that this resource relates to
				"command":                    name,
				"stok.goalspike.com/command": name,
			},
		},
		BinaryData: map[string][]byte{
			keyName: tarball,
		},
	}

	_, err := o.ConfigMapsClient(o.Namespace).Create(ctx, configMap, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	log.Debugf("Created config map %s/%s\n", o.Namespace, name)

	return nil
}
