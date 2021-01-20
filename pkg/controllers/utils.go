package controllers

import (
	"io/ioutil"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func requestFromObject(obj client.Object) reconcile.Request {
	return reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      obj.GetName(),
			Namespace: obj.GetNamespace(),
		},
	}
}

func readFile(path string) []byte {
	data, _ := ioutil.ReadFile(path)
	return data
}

func makeCopyOfMap(orig map[string]string) map[string]string {
	cp := make(map[string]string)
	for k, v := range orig {
		cp[k] = v
	}
	return cp
}
