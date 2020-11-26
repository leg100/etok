package launcher

import (
	"context"

	"github.com/leg100/stok/api/stok.goalspike.com/v1alpha1"
	"github.com/leg100/stok/pkg/labels"
	"github.com/leg100/stok/pkg/log"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (o *LauncherOptions) createRun(ctx context.Context, name, configMapName string, isTTY bool, relPathToRoot string) (*v1alpha1.Run, error) {
	run := &v1alpha1.Run{}
	run.SetNamespace(o.Namespace)
	run.SetName(name)

	// Set stok's common labels
	labels.SetCommonLabels(run)
	// Permit filtering runs by command
	labels.SetLabel(run, labels.Command(o.Command))
	// Permit filtering runs by workspace
	labels.SetLabel(run, labels.Workspace(o.Workspace))
	// Permit filtering stok resources by component
	labels.SetLabel(run, labels.RunComponent)

	run.SetWorkspace(o.Workspace)

	run.SetCommand(o.Command)
	run.SetArgs(o.args)
	run.SetDebug(o.Debug)
	run.SetConfigMap(configMapName)
	run.SetConfigMapKey(v1alpha1.RunDefaultConfigMapKey)
	run.SetConfigMapPath(relPathToRoot)

	if isTTY {
		run.AttachSpec.Handshake = true
		run.AttachSpec.HandshakeTimeout = o.HandshakeTimeout.String()
	}

	run, err := o.RunsClient(o.Namespace).Create(ctx, run, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	o.createdRun = true
	log.Debugf("created run %s/%s\n", o.Namespace, o.RunName)

	return run, nil
}

func (o *LauncherOptions) createConfigMap(ctx context.Context, tarball []byte, name, keyName string) error {
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: o.Namespace,
		},
		BinaryData: map[string][]byte{
			keyName: tarball,
		},
	}

	// Set stok's common labels
	labels.SetCommonLabels(configMap)
	// Permit filtering archives by command
	labels.SetLabel(configMap, labels.Command(o.Command))
	// Permit filtering archives by workspace
	labels.SetLabel(configMap, labels.Workspace(o.Workspace))
	// Permit filtering stok resources by component
	labels.SetLabel(configMap, labels.RunComponent)

	_, err := o.ConfigMapsClient(o.Namespace).Create(ctx, configMap, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	o.createdArchive = true
	log.Debugf("Created config map %s/%s\n", o.Namespace, name)

	return nil
}
