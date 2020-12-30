package k8s

import (
	"context"

	"github.com/leg100/etok/pkg/k8s/etokclient"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
)

type PodListWatcher struct {
	Client    kubernetes.Interface
	Namespace string
	Name      string
}

func (plw *PodListWatcher) List(options metav1.ListOptions) (runtime.Object, error) {
	setNameSelector(&options, plw.Name)
	return plw.Client.CoreV1().Pods(plw.Namespace).List(context.TODO(), options)
}

func (plw *PodListWatcher) Watch(options metav1.ListOptions) (watch.Interface, error) {
	setNameSelector(&options, plw.Name)
	return plw.Client.CoreV1().Pods(plw.Namespace).Watch(context.TODO(), options)
}

type WorkspaceListWatcher struct {
	Client    etokclient.Interface
	Namespace string
	Name      string
}

func (lw *WorkspaceListWatcher) List(options metav1.ListOptions) (runtime.Object, error) {
	setNameSelector(&options, lw.Name)
	return lw.Client.EtokV1alpha1().Workspaces(lw.Namespace).List(context.TODO(), options)
}

func (lw *WorkspaceListWatcher) Watch(options metav1.ListOptions) (watch.Interface, error) {
	setNameSelector(&options, lw.Name)
	return lw.Client.EtokV1alpha1().Workspaces(lw.Namespace).Watch(context.TODO(), options)
}

type RunListWatcher struct {
	Client    etokclient.Interface
	Namespace string
	Name      string
}

func (lw *RunListWatcher) List(options metav1.ListOptions) (runtime.Object, error) {
	setNameSelector(&options, lw.Name)
	return lw.Client.EtokV1alpha1().Runs(lw.Namespace).List(context.TODO(), options)
}

func (lw *RunListWatcher) Watch(options metav1.ListOptions) (watch.Interface, error) {
	setNameSelector(&options, lw.Name)
	return lw.Client.EtokV1alpha1().Runs(lw.Namespace).Watch(context.TODO(), options)
}

func setNameSelector(options *metav1.ListOptions, name string) {
	options.FieldSelector = fields.OneTermEqualSelector("metadata.name", name).String()
}
