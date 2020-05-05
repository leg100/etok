package dependents

import (
	"context"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// k8s calls 'controlled' objs 'dependents'...
type DependentMgr struct {
	Owner     metav1.Object
	Name      string
	Namespace string
	Client    client.Client
	Scheme    *runtime.Scheme
	Labels    map[string]string
}

func NewDependentMgr(owner metav1.Object, client client.Client, scheme *runtime.Scheme, name, namespace string, labels map[string]string) *DependentMgr {
	return &DependentMgr{
		Owner:     owner,
		Labels:    labels,
		Scheme:    scheme,
		Client:    client,
		Namespace: namespace,
		Name:      name,
	}
}

func (dm *DependentMgr) GetOrCreate(d Dependent) (bool, error) {
	err := dm.Client.Get(context.TODO(), types.NamespacedName{Name: dm.Name, Namespace: dm.Namespace}, d.GetRuntimeObj())
	if err != nil && errors.IsNotFound(err) {
		return dm.Create(d)
	} else if err != nil {
		return false, err
	}
	return false, nil
}

func (dm *DependentMgr) Create(d Dependent) (bool, error) {
	if err := d.Construct(); err != nil {
		return false, err
	}

	d.SetName(dm.Name)
	d.SetNamespace(dm.Namespace)
	d.SetLabels(dm.Labels)

	// Set Command instance as the owner and controller
	if err := controllerutil.SetControllerReference(dm.Owner, d, dm.Scheme); err != nil {
		return false, err
	}

	err := dm.Client.Create(context.TODO(), d.GetRuntimeObj())
	// ignore error wherein two reconciles in quick succession try to create obj
	if err != nil && errors.IsAlreadyExists(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}

	return true, nil
}
