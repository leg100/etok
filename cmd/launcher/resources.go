package launcher

import (
	"context"

	"github.com/leg100/etok/api/etok.dev/v1alpha1"
	"github.com/leg100/etok/pkg/labels"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

func (o *LauncherOptions) createRun(ctx context.Context, name, configMapName string, isTTY bool, relPathToRoot string) (*v1alpha1.Run, error) {
	run := &v1alpha1.Run{}
	run.SetNamespace(o.Namespace)
	run.SetName(name)

	// Set etok's common labels
	labels.SetCommonLabels(run)
	// Permit filtering runs by command
	labels.SetLabel(run, labels.Command(o.Command))
	// Permit filtering runs by workspace
	labels.SetLabel(run, labels.Workspace(o.Workspace))
	// Permit filtering etok resources by component
	labels.SetLabel(run, labels.RunComponent)

	run.SetWorkspace(o.Workspace)

	run.SetCommand(o.Command)
	run.SetArgs(o.args)
	run.SetConfigMap(configMapName)
	run.SetConfigMapKey(v1alpha1.RunDefaultConfigMapKey)
	run.SetConfigMapPath(relPathToRoot)

	run.Verbosity = o.Verbosity

	if isTTY {
		run.AttachSpec.Handshake = true
		run.AttachSpec.HandshakeTimeout = o.HandshakeTimeout.String()
	}

	run, err := o.RunsClient(o.Namespace).Create(ctx, run, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	o.createdRun = true
	klog.V(1).Infof("created run %s\n", klog.KObj(run))

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

	// Set etok's common labels
	labels.SetCommonLabels(configMap)
	// Permit filtering archives by command
	labels.SetLabel(configMap, labels.Command(o.Command))
	// Permit filtering archives by workspace
	labels.SetLabel(configMap, labels.Workspace(o.Workspace))
	// Permit filtering etok resources by component
	labels.SetLabel(configMap, labels.RunComponent)

	_, err := o.ConfigMapsClient(o.Namespace).Create(ctx, configMap, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	o.createdArchive = true
	klog.V(1).Infof("created config map %s\n", klog.KObj(configMap))

	return nil
}
